package domain

import (
	"errors"
	"time"
)

type Kind string

const (
	Banner     Kind = "banners"
	Navigation Kind = "navigation"
	Hero       Kind = "heroes"
	Product    Kind = "products"
	Solution   Kind = "solutions"
	Case       Kind = "cases"
	Service    Kind = "services"
	Security   Kind = "security"
	Pricing    Kind = "pricing"
	Resource   Kind = "resources"
	Footer     Kind = "footer"
)

func Kinds() []Kind {
	return []Kind{Banner, Navigation, Hero, Product, Solution, Case, Service, Security, Pricing, Resource, Footer}
}

func (k Kind) Valid() bool {
	for _, candidate := range Kinds() {
		if k == candidate {
			return true
		}
	}
	return false
}

type PublicationStatus string

const (
	Draft     PublicationStatus = "draft"
	Published PublicationStatus = "published"
	Archived  PublicationStatus = "archived"
)

var ErrNotPublished = errors.New("no published site")

type Link struct {
	Label    string `json:"label"`
	URL      string `json:"url"`
	Children []Link `json:"children,omitempty"`
}

type Image struct {
	URL string `json:"url"`
	Alt string `json:"alt"`
}

type Brand struct {
	Name    string `json:"name"`
	Logo    *Image `json:"logo,omitempty"`
	Tagline string `json:"tagline"`
}

// Price uses integer minor units to avoid floating point money rounding.
type Price struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
	Interval    string `json:"interval"`
}

type Content struct {
	ID       string   `json:"id"`
	Kind     Kind     `json:"kind"`
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Position int      `json:"position"`
	Image    *Image   `json:"image,omitempty"`
	CTA      *Link    `json:"cta,omitempty"`
	Links    []Link   `json:"links"`
	Features []string `json:"features"`
	Price    *Price   `json:"price,omitempty"`
}

// Each release is a complete snapshot; editing a draft cannot change public data.
type Site struct {
	ReleaseID   string             `json:"release_id"`
	Version     int                `json:"version"`
	PublishedAt time.Time          `json:"published_at"`
	Brand       Brand              `json:"brand"`
	Sections    map[Kind][]Content `json:"sections"`
}

func EmptySections() map[Kind][]Content {
	sections := make(map[Kind][]Content, len(Kinds()))
	for _, kind := range Kinds() {
		sections[kind] = []Content{}
	}
	return sections
}
