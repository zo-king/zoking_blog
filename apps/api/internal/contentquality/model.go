package contentquality

import "github.com/zo-king/zoking_blog/apps/api/internal/model"

func EvaluatePost(post model.Post) Report {
	return Evaluate(Document{
		Kind:           "post",
		Title:          post.Title,
		Slug:           post.Slug,
		Summary:        post.Summary,
		ContentMD:      post.ContentMD,
		Visibility:     post.Visibility,
		SEOTitle:       post.SEOTitle,
		SEODescription: post.SEODescription,
		HasCover:       post.CoverMediaID != nil,
		CategoryCount:  len(post.Categories),
		TagCount:       len(post.Tags),
		SeriesSelected: post.SeriesID != nil,
		SeriesOrderSet: post.SeriesOrder != nil,
		SeriesOrder:    intValue(post.SeriesOrder),
	})
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func EvaluatePage(page model.Page) Report {
	return Evaluate(Document{
		Kind:           "page",
		Title:          page.Title,
		Slug:           page.Slug,
		Summary:        page.Summary,
		ContentMD:      page.ContentMD,
		Visibility:     page.Visibility,
		SEOTitle:       page.SEOTitle,
		SEODescription: page.SEODescription,
		ShowInMenu:     page.ShowInMenu,
		MenuIcon:       page.MenuIcon,
	})
}
