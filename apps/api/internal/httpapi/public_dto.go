package httpapi

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
)

const (
	publicListLimit    = 100
	publicCommentLimit = 100
	publicSeriesLimit  = 100
)

type publicMediaDTO struct {
	ID        uuid.UUID `json:"id"`
	MimeType  string    `json:"mime_type"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	PublicURL string    `json:"public_url"`
}

func newPublicMediaDTO(media *model.MediaAsset) *publicMediaDTO {
	if media == nil {
		return nil
	}
	return &publicMediaDTO{ID: media.ID, MimeType: media.MimeType, Width: media.Width, Height: media.Height, PublicURL: media.PublicURL}
}

func parsePublicPagination(c *gin.Context, pageSize int) (paginationQuery, bool) {
	page, ok := parsePositiveQueryInt(c, "page", 1, 1_000_000)
	if !ok {
		return paginationQuery{}, false
	}
	requestedPageSize, ok := parsePositiveQueryInt(c, "page_size", pageSize, pageSize)
	if !ok {
		return paginationQuery{}, false
	}
	return paginationQuery{
		Page:     page,
		PageSize: requestedPageSize,
		Offset:   (page - 1) * requestedPageSize,
	}, true
}

func setPublicPaginationHeaders(c *gin.Context, total int64, query paginationQuery) {
	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(query.PageSize) - 1) / int64(query.PageSize)
	}
	hasMore := int64(query.Offset+query.PageSize) < total

	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	c.Header("X-Page", strconv.Itoa(query.Page))
	c.Header("X-Page-Size", strconv.Itoa(query.PageSize))
	c.Header("X-Total-Pages", strconv.FormatInt(totalPages, 10))
	c.Header("X-Has-More", strconv.FormatBool(hasMore))
	c.Header("Access-Control-Expose-Headers", "Link, X-Total-Count, X-Page, X-Page-Size, X-Total-Pages, X-Has-More")

	links := make([]string, 0, 2)
	if query.Page > 1 {
		links = append(links, publicPageLink(c, query.Page-1, query.PageSize, "prev"))
	}
	if hasMore {
		links = append(links, publicPageLink(c, query.Page+1, query.PageSize, "next"))
	}
	if len(links) > 0 {
		c.Header("Link", strings.Join(links, ", "))
	}
}

func publicPageLink(c *gin.Context, page, pageSize int, relation string) string {
	url := *c.Request.URL
	query := url.Query()
	query.Set("page", strconv.Itoa(page))
	query.Set("page_size", strconv.Itoa(pageSize))
	url.RawQuery = query.Encode()
	return "<" + url.RequestURI() + ">; rel=\"" + relation + "\""
}

type publicTaxonomyDTO struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	SortOrder   int        `json:"sort_order,omitempty"`
	Enabled     bool       `json:"enabled,omitempty"`
	Color       string     `json:"color,omitempty"`
}

func newPublicCategoryDTO(item model.Category) publicTaxonomyDTO {
	return publicTaxonomyDTO{ID: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, ParentID: item.ParentID, SortOrder: item.SortOrder, Enabled: item.Enabled}
}

func newPublicTagDTO(item model.Tag) publicTaxonomyDTO {
	return publicTaxonomyDTO{ID: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, Color: item.Color}
}

type publicSeriesDTO struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	CoverMedia  *publicMediaDTO `json:"cover_media,omitempty"`
	SortOrder   int             `json:"sort_order"`
	PostCount   int64           `json:"post_count"`
	Posts       []publicPostDTO `json:"posts,omitempty"`
}

type publicPostDTO struct {
	ID           uuid.UUID           `json:"id"`
	Title        string              `json:"title"`
	Slug         string              `json:"slug"`
	Summary      string              `json:"summary"`
	ContentMD    string              `json:"content_md,omitempty"`
	PublishedAt  *time.Time          `json:"published_at,omitempty"`
	AllowComment bool                `json:"allow_comment"`
	CoverMedia   *publicMediaDTO     `json:"cover_media,omitempty"`
	Series       *publicSeriesRefDTO `json:"series,omitempty"`
	Categories   []publicTaxonomyDTO `json:"categories,omitempty"`
	Tags         []publicTaxonomyDTO `json:"tags,omitempty"`
}

type publicSeriesRefDTO struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	CoverMedia  *publicMediaDTO `json:"cover_media,omitempty"`
}

func newPublicSeriesRefDTO(item *model.Series) *publicSeriesRefDTO {
	if item == nil {
		return nil
	}
	return &publicSeriesRefDTO{ID: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, CoverMedia: newPublicMediaDTO(item.CoverMedia)}
}

func newPublicPostDTO(item model.Post) publicPostDTO {
	categories := make([]publicTaxonomyDTO, 0, len(item.Categories))
	for _, category := range item.Categories {
		categories = append(categories, newPublicCategoryDTO(category))
	}
	tags := make([]publicTaxonomyDTO, 0, len(item.Tags))
	for _, tag := range item.Tags {
		tags = append(tags, newPublicTagDTO(tag))
	}
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].SortOrder != categories[j].SortOrder {
			return categories[i].SortOrder < categories[j].SortOrder
		}
		if categories[i].Name != categories[j].Name {
			return categories[i].Name < categories[j].Name
		}
		return categories[i].ID.String() < categories[j].ID.String()
	})
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Name != tags[j].Name {
			return tags[i].Name < tags[j].Name
		}
		return tags[i].ID.String() < tags[j].ID.String()
	})
	return publicPostDTO{ID: item.ID, Title: item.Title, Slug: item.Slug, Summary: item.Summary, ContentMD: item.ContentMD, PublishedAt: item.PublishedAt, AllowComment: item.AllowComment, CoverMedia: newPublicMediaDTO(item.CoverMedia), Series: newPublicSeriesRefDTO(item.Series), Categories: categories, Tags: tags}
}

func newPublicPostSummaryDTO(item model.Post) publicPostDTO {
	result := newPublicPostDTO(item)
	result.ContentMD = ""
	return result
}

func newPublicSeriesDTO(item model.Series) publicSeriesDTO {
	result := publicSeriesDTO{ID: item.ID, Name: item.Name, Slug: item.Slug, Description: item.Description, CoverMedia: newPublicMediaDTO(item.CoverMedia), SortOrder: item.SortOrder, PostCount: item.PostCount}
	if len(item.Posts) > 0 {
		result.Posts = make([]publicPostDTO, 0, len(item.Posts))
		for _, post := range item.Posts {
			result.Posts = append(result.Posts, newPublicPostSummaryDTO(post))
		}
	}
	return result
}

type publicPageDTO struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Summary     string     `json:"summary"`
	ContentMD   string     `json:"content_md"`
	ShowInMenu  bool       `json:"show_in_menu"`
	MenuWeight  int        `json:"menu_weight"`
	MenuIcon    string     `json:"menu_icon"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

func newPublicPageDTO(item model.Page) publicPageDTO {
	return publicPageDTO{ID: item.ID, Title: item.Title, Slug: item.Slug, Summary: item.Summary, ContentMD: item.ContentMD, ShowInMenu: item.ShowInMenu, MenuWeight: item.MenuWeight, MenuIcon: item.MenuIcon, PublishedAt: item.PublishedAt}
}
