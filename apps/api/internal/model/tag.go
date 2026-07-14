package model

type Tag struct {
	Base
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func (Tag) TableName() string {
	return "tags"
}
