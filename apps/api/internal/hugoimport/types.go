package hugoimport

import (
	"context"
	"io"
	"time"
)

type TaxonomyMap struct {
	Categories map[string]string `yaml:"categories"`
	Tags       map[string]string `yaml:"tags"`
}

type FrontMatter struct {
	Title       string   `yaml:"title"`
	Slug        string   `yaml:"slug"`
	Date        string   `yaml:"date"`
	Description string   `yaml:"description"`
	Image       string   `yaml:"image"`
	Categories  []string `yaml:"categories"`
	Tags        []string `yaml:"tags"`
}

type Article struct {
	FrontMatter
	PublishedAt time.Time
	Body        string
	SourcePath  string
	Assets      []Asset
}

type Asset struct {
	StaticURL  string
	ManagedURL string
	Path       string
	Checksum   string
	Filename   string
}

type Options struct {
	SiteDir     string
	TaxonomyMap string
	APIBase     string
	MediaBase   string
	Email       string
	Password    string
	Publish     bool
	PollEvery   time.Duration
	JobTimeout  time.Duration
	Out         io.Writer
}

type Result struct {
	Created   int
	Updated   int
	Unchanged int
	Uploaded  int
	Reused    int
	Published int
}

type Runner interface {
	Run(context.Context, Options) (Result, error)
}
