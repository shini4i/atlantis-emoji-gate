package processor

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/shini4i/atlantis-emoji-gate/internal/client"
	"github.com/shini4i/atlantis-emoji-gate/internal/config"
)

//go:generate go tool mockgen -destination=mocks/mock_processor.go -package=mocks . Processor

// Processor defines the contract for checking approvals against a CODEOWNERS file.
type Processor interface {
	CheckApproval(codeowners io.Reader, reaction *client.AwardEmoji, cfg config.GitlabConfig) (bool, error)
}

// approvalProcessor provides a concrete implementation of the Processor interface.
type approvalProcessor struct{}

// NewProcessor creates and returns a new instance of the approval processor.
func NewProcessor() Processor {
	return &approvalProcessor{}
}

// escapedWhitespace matches a backslash followed by whitespace, which GitLab
// reads as a literal space inside a path.
var escapedWhitespace = regexp.MustCompile(`\\[ \t\n\r\f\v]+`)

// isRubySpace reports whether r is whitespace as Ruby's regexp `\s` defines it (ASCII only).
func isRubySpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// splitEntry splits a CODEOWNERS line into its path pattern and owners.
// The pattern ends at the first ASCII whitespace not preceded by a backslash, as in GitLab.
// owners is nil when the line holds only a pattern.
func splitEntry(line string) (pattern string, owners []string) {
	for i, r := range line {
		if isRubySpace(r) && i > 0 && line[i-1] != '\\' {
			return line[:i], strings.FieldsFunc(line[i:], isRubySpace)
		}
	}
	return line, nil
}

// normalizePattern rewrites a pattern as GitLab's normalize_pattern does
// (ee/lib/gitlab/code_owners/file.rb): `*` owns everything, a leading `\#` and each `\ ` are
// unescaped, no leading `/` matches at any depth, a trailing `/` covers the whole tree.
func normalizePattern(pattern string) string {
	if pattern == "*" {
		return "/**/*"
	}

	if strings.HasPrefix(pattern, `\#`) {
		pattern = pattern[1:]
	}
	pattern = escapedWhitespace.ReplaceAllString(pattern, " ")

	if !strings.HasPrefix(pattern, "/") {
		pattern = "/**/" + pattern
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**/*"
	}
	return pattern
}

// rewriteClassNegation turns each unescaped `[!` into `[^`, the only negation path.Match knows.
func rewriteClassNegation(segment string) string {
	var b strings.Builder
	for i := 0; i < len(segment); i++ {
		b.WriteByte(segment[i])
		switch {
		case segment[i] == '\\' && i+1 < len(segment):
			i++
			b.WriteByte(segment[i])
		case segment[i] == '[' && i+1 < len(segment) && segment[i+1] == '!':
			i++
			b.WriteByte('^')
		}
	}
	return b.String()
}

// ownsDirectory reports whether a normalized pattern owns dir ("." is the repository root).
// GitLab matches file paths with File.fnmatch(FNM_PATHNAME | FNM_DOTMATCH), so a pattern
// owns dir when it matches any file placed directly in it.
// It returns an error when a pattern segment is malformed.
func ownsDirectory(pattern, dir string) (bool, error) {
	segments := strings.Split(pattern, "/")
	for i, segment := range segments {
		segment = rewriteClassNegation(segment)
		if _, err := path.Match(segment, ""); err != nil {
			return false, err
		}
		segments[i] = segment
	}

	dirSegments := []string{""}
	dir = strings.Trim(path.Clean(dir), "/")
	if dir != "." && dir != "" {
		dirSegments = append(dirSegments, strings.Split(dir, "/")...)
	}

	return matchSegments(segments, dirSegments, map[[2]int]bool{}), nil
}

// matchSegments matches validated pattern segments against dirs followed by any file name.
// A non-last `**` matches zero or more directories; a last `**` behaves like `*`.
// failed caches (pattern, dirs) suffix lengths that did not match, so repeated `**` stay cheap.
func matchSegments(pattern, dirs []string, failed map[[2]int]bool) bool {
	key := [2]int{len(pattern), len(dirs)}
	if failed[key] {
		return false
	}

	var matched bool
	switch {
	case len(dirs) == 0:
		for len(pattern) > 1 && pattern[0] == "**" {
			pattern = pattern[1:]
		}
		matched = len(pattern) == 1 && strings.Trim(pattern[0], "*") == ""
	case len(pattern) == 0:
		matched = false
	case pattern[0] == "**" && len(pattern) > 1:
		matched = matchSegments(pattern[1:], dirs, failed) || matchSegments(pattern, dirs[1:], failed)
	default:
		segmentMatched, _ := path.Match(pattern[0], dirs[0])
		matched = segmentMatched && matchSegments(pattern[1:], dirs[1:], failed)
	}

	if !matched {
		failed[key] = true
	}
	return matched
}

// CheckApproval determines if a given user reaction constitutes a valid approval.
// Rules use GitLab's CODEOWNERS path syntax; the last rule owning cfg.TerraformPath wins,
// and lines without owners are ignored. It returns an error when the content cannot be
// read or holds a malformed pattern.
func (p *approvalProcessor) CheckApproval(codeowners io.Reader, reaction *client.AwardEmoji, cfg config.GitlabConfig) (bool, error) {
	if reaction.Name != cfg.ApproveEmoji {
		return false, nil
	}
	if !cfg.Insecure && reaction.User.Username == cfg.MrAuthor {
		return false, nil
	}

	var applicableOwners []string
	scanner := bufio.NewScanner(codeowners)

	for scanner.Scan() {
		line := strings.TrimFunc(scanner.Text(), isRubySpace)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pathPattern, owners := splitEntry(line)
		if len(owners) == 0 {
			continue
		}

		matched, err := ownsDirectory(normalizePattern(pathPattern), cfg.TerraformPath)
		if err != nil {
			return false, fmt.Errorf("invalid pattern '%s' in CODEOWNERS: %w", pathPattern, err)
		}

		if matched {
			applicableOwners = make([]string, 0, len(owners))
			for _, owner := range owners {
				applicableOwners = append(applicableOwners, strings.TrimPrefix(owner, "@"))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading CODEOWNERS content: %w", err)
	}

	for _, owner := range applicableOwners {
		if owner == reaction.User.Username {
			return true, nil
		}
	}

	return false, nil
}
