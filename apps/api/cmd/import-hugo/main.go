package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/zo-king/zoking_blog/apps/api/internal/hugoimport"
)

func main() {
	options := hugoimport.Options{}
	flag.StringVar(&options.SiteDir, "site-dir", "../site", "Hugo site root")
	flag.StringVar(&options.TaxonomyMap, "taxonomy-map", "../../scripts/content/taxonomy-map.yaml", "explicit taxonomy mapping file")
	flag.StringVar(&options.APIBase, "api-base", env("IMPORT_HUGO_API_BASE", "http://localhost:18080"), "Admin API base URL")
	flag.StringVar(&options.MediaBase, "media-base", env("IMPORT_HUGO_MEDIA_BASE", ""), "publisher-managed media URL base (defaults to API base + /media-files)")
	flag.StringVar(&options.Email, "email", env("IMPORT_HUGO_EMAIL", env("SEED_ADMIN_EMAIL", "admin@zoking.local")), "Admin login email")
	flag.StringVar(&options.Password, "password", env("IMPORT_HUGO_PASSWORD", env("SEED_ADMIN_PASSWORD", "")), "Admin login password")
	flag.BoolVar(&options.Publish, "publish", true, "publish each imported post and wait for its job")
	flag.DurationVar(&options.PollEvery, "poll-every", time.Second, "publish job polling interval")
	flag.DurationVar(&options.JobTimeout, "job-timeout", 3*time.Minute, "timeout for each publish job")
	flag.Parse()
	if options.MediaBase == "" {
		options.MediaBase = strings.TrimRight(options.APIBase, "/") + "/media-files"
	}
	options.Out = os.Stdout

	result, err := (hugoimport.Importer{}).Run(context.Background(), options)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("import complete: created=%d updated=%d unchanged=%d uploaded=%d reused=%d published=%d\n",
		result.Created, result.Updated, result.Unchanged, result.Uploaded, result.Reused, result.Published)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
