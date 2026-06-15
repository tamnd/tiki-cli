package tiki

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Tiki driver.
type Domain struct{}

func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "tiki",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "tiki",
			Short:  "Read public Tiki (tiki.vn) product listings, reviews, and categories.",
			Long: `Read public Tiki (tiki.vn) product listings, reviews, and categories.

tiki reads from the Tiki public JSON API — no API key, no browser required.
Returns clean JSON records ready for jq, sqlite-utils, and shell pipelines.`,
			Site: Host,
			Repo: "https://github.com/tamnd/tiki-cli",
		},
	}
}

func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "product", Group: "read", Single: true,
		URIType: "product", Resolver: true,
		Summary: "Fetch full details for a Tiki product by ID",
		Args:    []kit.Arg{{Name: "id", Help: "Tiki product ID"}}}, getProduct)

	kit.Handle(app, kit.OpMeta{Name: "products", Group: "read", List: true,
		URIType: "product",
		Summary: "List Tiki products for a category"}, listProducts)

	kit.Handle(app, kit.OpMeta{Name: "reviews", Group: "read", List: true,
		URIType: "review",
		Summary: "List customer reviews for a product",
		Args:    []kit.Arg{{Name: "product_id", Help: "Tiki product ID"}}}, listReviews)

	kit.Handle(app, kit.OpMeta{Name: "categories", Group: "read", List: true,
		URIType: "category",
		Summary: "List top-level Tiki product categories"}, listCategories)
}

func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClientWithConfig(c), nil
}

type productInput struct {
	ID     string  `kit:"arg"   help:"Tiki product ID"`
	Client *Client `kit:"inject"`
}

type productsInput struct {
	Category string  `kit:"flag" help:"category ID to filter by"`
	Sort     string  `kit:"flag" help:"sort: top_seller, newest, price_asc, price_desc"`
	Limit    int     `kit:"flag,inherit" help:"max results"`
	Client   *Client `kit:"inject"`
}

type reviewsInput struct {
	ProductID string  `kit:"arg"          help:"Tiki product ID"`
	Limit     int     `kit:"flag,inherit" help:"max results"`
	Client    *Client `kit:"inject"`
}

type categoriesInput struct {
	Client *Client `kit:"inject"`
}

func getProduct(ctx context.Context, in productInput, emit func(*Product) error) error {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return errs.Usage("product id is required")
	}
	p, err := in.Client.GetProduct(ctx, id)
	if err != nil {
		return err
	}
	return emit(p)
}

func listProducts(ctx context.Context, in productsInput, emit func(*Product) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 40
	}
	products, err := in.Client.ListProducts(ctx, in.Category, in.Sort, limit)
	if err != nil {
		return err
	}
	for _, p := range products {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

func listReviews(ctx context.Context, in reviewsInput, emit func(*Review) error) error {
	pid := strings.TrimSpace(in.ProductID)
	if pid == "" {
		return errs.Usage("product_id is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	reviews, err := in.Client.ListReviews(ctx, pid, limit)
	if err != nil {
		return err
	}
	for _, r := range reviews {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func listCategories(ctx context.Context, in categoriesInput, emit func(*Category) error) error {
	cats, err := in.Client.ListCategories(ctx)
	if err != nil {
		return err
	}
	for _, cat := range cats {
		if err := emit(cat); err != nil {
			return err
		}
	}
	return nil
}

// Classify turns a Tiki URL or product ID into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty Tiki reference")
	}
	if strings.Contains(input, "tiki.vn/") {
		id = extractProductID(input)
		if id != "" {
			return "product", id, nil
		}
	}
	if isDigits(input) {
		return "product", input, nil
	}
	return "", "", errs.Usage("unrecognized Tiki reference: %q", input)
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "product":
		return baseURL + "/p" + id + ".html", nil
	case "category":
		return baseURL + "/c" + id, nil
	default:
		return "", errs.Usage("tiki has no resource type %q", uriType)
	}
}

// extractProductID extracts the numeric product ID from a Tiki product URL.
// Pattern: https://tiki.vn/{url_key}/p{id}.html
func extractProductID(rawURL string) string {
	// Look for /p{digits}.html pattern
	idx := strings.LastIndex(rawURL, "/p")
	if idx < 0 {
		return ""
	}
	rest := rawURL[idx+2:]
	end := strings.Index(rest, ".")
	if end < 0 {
		end = strings.Index(rest, "?")
	}
	if end < 0 {
		end = len(rest)
	}
	id := rest[:end]
	if isDigits(id) {
		return id
	}
	return ""
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
