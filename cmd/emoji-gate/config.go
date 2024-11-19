package main

import envConfig "github.com/caarlos0/env/v11"

type GitlabConfig struct {
	Url            string `env:"ATLANTIS_GITLAB_HOSTNAME,required,notEmpty"`
	Token          string `env:"ATLANTIS_GITLAB_TOKEN,required,notEmpty"`
	ApproveEmoji   string `env:"APPROVE_EMOJI,notEmpty" envDefault:"thumbsup"`
	BaseRepoOwner  string `env:"BASE_REPO_OWNER,required,notEmpty"`
	BaseRepoName   string `env:"BASE_REPO_NAME,required,notEmpty"`
	PullRequestID  int    `env:"PULL_NUM,required,notEmpty"`
	CodeOwnersPath string `env:"CODEOWNERS_PATH,notEmpty" envDefault:"CODEOWNERS"`
	MrAuthor       string `env:"PULL_AUTHOR,required,notEmpty"`
	Insecure       bool   `env:"INSECURE,notEmpty" envDefault:"false"` // If MR author allowed to approve his own MR
}

func NewGitlabConfig() GitlabConfig {
	if cfg, err := envConfig.ParseAs[GitlabConfig](); err != nil {
		panic(err)
	} else {
		return cfg
	}
}
