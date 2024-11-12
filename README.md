<div align="center">

# atlantis-emoji-gate

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/shini4i/atlantis-emoji-gate)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/shini4i/atlantis-emoji-gate)
[![Go Report Card](https://goreportcard.com/badge/github.com/shini4i/atlantis-emoji-gate)](https://goreportcard.com/report/github.com/shini4i/atlantis-emoji-gate)
![GitHub](https://img.shields.io/github/license/shini4i/atlantis-emoji-gate)


</div>

> [!WARNING]
> This project is in the early stages of development and is not yet ready for production use.

## General information

`atlantis-emoji-gate` is a tool designed to work with Atlantis on GitLab Community Edition (CE) to ensure that a specific emoji reaction is present on a GitLab merge request.

This acts as a replacement for mandatory MR approval (which does not work on CE), and can be used to ensure that a specific person has reviewed the MR before `atlantis apply` is allowed to run.

## Configuration

`atlantis-emoji-gate` is configured using environment variables. The following variables are available:

- `ATLANTIS_GITLAB_HOSTNAME` - The hostname of the GitLab instance to connect to (should be already present in the Atlantis environment)
- `ATLANTIS_GITLAB_TOKEN` - The token to use to authenticate with the GitLab instance (should be already present in the Atlantis environment)
- `APPROVE_EMOJI` - The emoji that must be present on the MR for `atlantis apply` to be allowed to run (default: `thumbsup`)
- `CODEOWNERS_PATH` - The path to the CODEOWNERS file in the repository (default: `CODEOWNERS`)

The remaining environment variables are set dynamically by Atlantis and should not be set manually.

At this early stage, only owners of the whole repository are supported.

CODEOWNERS file example:
```
* @username
```

Workflow example:
```yaml
workflows:
  default:
    plan:
      steps:
        - init
        - plan
    apply:
      steps:
        - run: emoji-gate
        - apply
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.
