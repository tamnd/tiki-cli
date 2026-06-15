// Package tiki is the library behind the tiki command line:
// the HTTP client, JSON API parsing, and typed data models for Tiki
// (tiki.vn), Vietnam's leading e-commerce marketplace.
//
// Tiki exposes a public JSON REST API for product listings, product details,
// customer reviews, and the category tree. No API key is required.
// Product URLs follow the pattern: https://tiki.vn/{url_key}/p{id}.html.
package tiki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Host is the canonical site hostname.
const Host = "tiki.vn"

// baseURL is the site root.
const baseURL = "https://tiki.vn"

// apiBase is the root of the public JSON API.
const apiBase = "https://tiki.vn/api/v2"

// DefaultUserAgent identifies this client to Tiki.
const DefaultUserAgent = "tiki-cli/0.1.0 (+https://github.com/tamnd/tiki-cli)"

// Config holds the tunable knobs for the HTTP client.
type Config struct {
	BaseURL   string
	APIBase   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   baseURL,
		APIBase:   apiBase,
		Rate:      time.Second,
		Retries:   3,
		Timeout:   30 * time.Second,
		UserAgent: DefaultUserAgent,
	}
}

// Client talks to the Tiki API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client from DefaultConfig.
func NewClient() *Client { return NewClientWithConfig(DefaultConfig()) }

// NewClientWithConfig returns a Client built from cfg.
func NewClientWithConfig(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// Get fetches rawURL and returns the body bytes, pacing and retrying on transient errors.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- wire types (match Tiki API JSON field names) ---

type wireProductList struct {
	Data   []wireProduct `json:"data"`
	Paging wirePaging    `json:"paging"`
}

type wirePaging struct {
	Total   int `json:"total"`
	PerPage int `json:"per_page"`
	Page    int `json:"current_page"`
}

type wireProduct struct {
	ID              int64          `json:"id"`
	SKU             string         `json:"sku"`
	Name            string         `json:"name"`
	URLKey          string         `json:"url_key"`
	ShortDesc       string         `json:"short_description"`
	Price           float64        `json:"price"`
	ListPrice       float64        `json:"list_price"`
	DiscountRate    int            `json:"discount_rate"`
	RatingAverage   float64        `json:"rating_average"`
	ReviewCount     int            `json:"review_count"`
	SoldCount       int64          `json:"all_time_quantity_sold"`
	QuantitySold    int64          `json:"quantity_sold"`
	BrandID         int64          `json:"brand_id"`
	BrandName       string         `json:"brand_name"`
	SellerID        int64          `json:"seller_id"`
	SellerName      string         `json:"seller_name"`
	IsOfficialStore bool           `json:"is_official_store"`
	IsTikiTrading   bool           `json:"is_tiki_trading"`
	FulfillmentType string         `json:"fulfillment_type"`
	Categories      []wireCategory `json:"breadcrumbs"`
	Images          []wireImage    `json:"images"`
	Badges          []wireBadge    `json:"badges_v3"`
	Specifications  []wireSpec     `json:"specifications"`
	StockItemQty    int            `json:"stock_item"`
	HasWarranty     bool           `json:"has_warranty"`
	WarrantyPeriod  string         `json:"warranty_period"`
}

type wireCategory struct {
	ID   int64  `json:"category_id"`
	Name string `json:"name"`
}

type wireImage struct {
	BaseURL string `json:"base_url"`
}

type wireBadge struct {
	Text string `json:"text"`
}

type wireSpec struct {
	Name       string         `json:"name"`
	Attributes []wireSpecAttr `json:"attributes"`
}

type wireSpecAttr struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type wireReviewList struct {
	Data   []wireReview `json:"data"`
	Paging wirePaging   `json:"paging"`
}

type wireReview struct {
	ID           int64           `json:"id"`
	ProductID    int64           `json:"product_id"`
	Rating       int             `json:"rating"`
	Title        string          `json:"title"`
	Content      string          `json:"content"`
	CustomerID   int64           `json:"created_by_id"`
	CustomerName string          `json:"created_by_name"`
	CreatedAt    int64           `json:"created_at"`
	HelpfulCount int             `json:"thank_count"`
	Images       []wireImage     `json:"images"`
	Attributes   json.RawMessage `json:"attributes"`
}

type wireCategoryTree struct {
	Data []wireCatNode `json:"data"`
}

type wireCatNode struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	URLKey   string         `json:"url_key"`
	Children []wireCatNode  `json:"children"`
}

// --- public types ---

// Product is one Tiki product from the API.
type Product struct {
	ID              string  `json:"id"                         kit:"id" table:"id"`
	SKU             string  `json:"sku,omitempty"                       table:"sku"`
	Name            string  `json:"name"                                table:"name"`
	URL             string  `json:"url,omitempty"                       table:"url,url"`
	ShortDesc       string  `json:"short_description,omitempty"         table:"-"`
	Price           float64 `json:"price"                               table:"price"`
	ListPrice       float64 `json:"list_price,omitempty"                table:"list_price"`
	DiscountRate    int     `json:"discount_rate,omitempty"             table:"discount_rate"`
	BrandName       string  `json:"brand_name,omitempty"                table:"brand_name"`
	SellerName      string  `json:"seller_name,omitempty"               table:"seller_name"`
	IsOfficialStore bool    `json:"is_official_store,omitempty"         table:"official_store"`
	IsTikiTrading   bool    `json:"is_tiki_trading,omitempty"           table:"tiki_trading"`
	RatingAverage   float64 `json:"rating_average,omitempty"            table:"rating"`
	ReviewCount     int     `json:"review_count,omitempty"              table:"reviews"`
	SoldCount       int64   `json:"sold_count,omitempty"                table:"sold"`
	FulfillmentType string  `json:"fulfillment_type,omitempty"          table:"fulfillment"`
	WarrantyPeriod  string  `json:"warranty_period,omitempty"           table:"warranty"`
	FetchedAt       string  `json:"fetched_at,omitempty"                table:"fetched_at"`
}

// Review is one customer review for a Tiki product.
type Review struct {
	ID           string `json:"id"                    kit:"id" table:"id"`
	ProductID    string `json:"product_id"                      table:"product_id"`
	Rating       int    `json:"rating"                          table:"rating"`
	Title        string `json:"title,omitempty"                 table:"title"`
	Content      string `json:"content,omitempty"               table:"-"`
	CustomerName string `json:"customer_name,omitempty"         table:"customer_name"`
	CreatedAt    string `json:"created_at,omitempty"            table:"created_at"`
	HelpfulCount int    `json:"helpful_count,omitempty"         table:"helpful"`
	FetchedAt    string `json:"fetched_at,omitempty"            table:"fetched_at"`
}

// Category is one node in the Tiki category tree.
type Category struct {
	ID     string `json:"id"     kit:"id" table:"id"`
	Name   string `json:"name"            table:"name"`
	URLKey string `json:"url_key"         table:"url_key"`
	URL    string `json:"url"             table:"url,url"`
}

// --- client methods ---

// GetProduct fetches full details for a single product by ID.
func (c *Client) GetProduct(ctx context.Context, id string) (*Product, error) {
	apiURL := c.cfg.APIBase + "/products/" + id
	body, err := c.Get(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("product %s: %w", id, err)
	}
	var wire wireProduct
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("decode product %s: %w", id, err)
	}
	return productFromWire(wire, c.cfg.BaseURL), nil
}

// ListProducts fetches products for a category, sorted by the given field.
// sort is one of "top_seller", "newest", "price_asc", "price_desc". Empty uses default.
func (c *Client) ListProducts(ctx context.Context, categoryID string, sort string, limit int) ([]*Product, error) {
	if limit <= 0 {
		limit = 40
	}
	base := c.cfg.APIBase
	if base == "" {
		base = apiBase
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(min(limit, 40)))
	params.Set("page", "1")
	if categoryID != "" {
		params.Set("category", categoryID)
	}
	if sort != "" {
		params.Set("sort", sort)
	}
	apiURL := base + "/products?" + params.Encode()

	body, err := c.Get(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("products: %w", err)
	}

	var list wireProductList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}

	siteBase := c.cfg.BaseURL
	if siteBase == "" {
		siteBase = baseURL
	}
	out := make([]*Product, 0, len(list.Data))
	for _, w := range list.Data {
		if len(out) >= limit {
			break
		}
		out = append(out, productFromWire(w, siteBase))
	}
	return out, nil
}

// ListReviews fetches customer reviews for a product.
func (c *Client) ListReviews(ctx context.Context, productID string, limit int) ([]*Review, error) {
	if limit <= 0 {
		limit = 20
	}
	base := c.cfg.APIBase
	if base == "" {
		base = apiBase
	}

	params := url.Values{}
	params.Set("product_id", productID)
	params.Set("limit", strconv.Itoa(min(limit, 20)))
	params.Set("page", "1")
	params.Set("sort", "relevant")
	apiURL := base + "/reviews?" + params.Encode()

	body, err := c.Get(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("reviews for %s: %w", productID, err)
	}

	var list wireReviewList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decode reviews: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	out := make([]*Review, 0, len(list.Data))
	for _, w := range list.Data {
		if len(out) >= limit {
			break
		}
		out = append(out, reviewFromWire(w, productID, now))
	}
	return out, nil
}

// ListCategories fetches the top-level Tiki category tree.
func (c *Client) ListCategories(ctx context.Context) ([]*Category, error) {
	base := c.cfg.APIBase
	if base == "" {
		base = apiBase
	}
	apiURL := base + "/categories?include=children"
	body, err := c.Get(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}

	var tree wireCategoryTree
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("decode categories: %w", err)
	}

	siteBase := c.cfg.BaseURL
	if siteBase == "" {
		siteBase = baseURL
	}
	var out []*Category
	for _, node := range tree.Data {
		out = append(out, categoryFromWire(node, siteBase))
	}
	return out, nil
}

// --- wire → public conversions ---

func productFromWire(w wireProduct, siteBase string) *Product {
	if siteBase == "" {
		siteBase = baseURL
	}
	productURL := ""
	if w.URLKey != "" && w.ID > 0 {
		productURL = siteBase + "/" + w.URLKey + "/p" + strconv.FormatInt(w.ID, 10) + ".html"
	}
	return &Product{
		ID:              strconv.FormatInt(w.ID, 10),
		SKU:             w.SKU,
		Name:            w.Name,
		URL:             productURL,
		ShortDesc:       w.ShortDesc,
		Price:           w.Price,
		ListPrice:       w.ListPrice,
		DiscountRate:    w.DiscountRate,
		BrandName:       w.BrandName,
		SellerName:      w.SellerName,
		IsOfficialStore: w.IsOfficialStore,
		IsTikiTrading:   w.IsTikiTrading,
		RatingAverage:   w.RatingAverage,
		ReviewCount:     w.ReviewCount,
		SoldCount:       w.SoldCount,
		FulfillmentType: w.FulfillmentType,
		WarrantyPeriod:  w.WarrantyPeriod,
		FetchedAt:       time.Now().UTC().Format(time.RFC3339),
	}
}

func reviewFromWire(w wireReview, productID, now string) *Review {
	createdAt := ""
	if w.CreatedAt > 0 {
		createdAt = time.Unix(w.CreatedAt, 0).UTC().Format(time.RFC3339)
	}
	return &Review{
		ID:           strconv.FormatInt(w.ID, 10),
		ProductID:    productID,
		Rating:       w.Rating,
		Title:        strings.TrimSpace(w.Title),
		Content:      strings.TrimSpace(w.Content),
		CustomerName: strings.TrimSpace(w.CustomerName),
		CreatedAt:    createdAt,
		HelpfulCount: w.HelpfulCount,
		FetchedAt:    now,
	}
}

func categoryFromWire(node wireCatNode, siteBase string) *Category {
	return &Category{
		ID:     strconv.FormatInt(node.ID, 10),
		Name:   node.Name,
		URLKey: node.URLKey,
		URL:    siteBase + "/" + node.URLKey + "/c" + strconv.FormatInt(node.ID, 10),
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
