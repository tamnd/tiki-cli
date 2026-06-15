package tiki

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.APIBase = srv.URL + "/api/v2"
	cfg.Rate = 0
	cfg.Retries = 0
	cfg.Timeout = 5 * time.Second
	return NewClientWithConfig(cfg)
}

func sampleProductJSON(id int64, name string) string {
	return fmt.Sprintf(`{
		"id": %d,
		"sku": "SKU%d",
		"name": %q,
		"url_key": "san-pham-%d",
		"short_description": "Mô tả ngắn",
		"price": 150000,
		"list_price": 200000,
		"discount_rate": 25,
		"brand_name": "TestBrand",
		"seller_name": "TestSeller",
		"is_official_store": true,
		"is_tiki_trading": false,
		"rating_average": 4.5,
		"review_count": 100,
		"all_time_quantity_sold": 500,
		"fulfillment_type": "tiki_now",
		"warranty_period": "12 tháng"
	}`, id, id, name, id)
}

func sampleProductListJSON(n int) string {
	items := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			items += ","
		}
		items += fmt.Sprintf(`{
			"id": %d,
			"name": "Sản phẩm %d",
			"url_key": "san-pham-%d",
			"price": %d,
			"list_price": %d,
			"rating_average": 4.0,
			"review_count": %d
		}`, 100+i, i+1, 100+i, 150000*(i+1), 200000*(i+1), 50*(i+1))
	}
	return fmt.Sprintf(`{"data":[%s],"paging":{"total":%d,"per_page":40,"current_page":1}}`, items, n)
}

func sampleReviewListJSON(productID int64, n int) string {
	items := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			items += ","
		}
		items += fmt.Sprintf(`{
			"id": %d,
			"product_id": %d,
			"rating": %d,
			"title": "Đánh giá %d",
			"content": "Nội dung đánh giá số %d",
			"created_by_name": "Khách hàng %d",
			"created_at": 1718438400,
			"thank_count": %d
		}`, 200+i, productID, (i%5)+1, i+1, i+1, i+1, i*2)
	}
	return fmt.Sprintf(`{"data":[%s],"paging":{"total":%d,"per_page":20,"current_page":1}}`, items, n)
}

func sampleCategoryListJSON() string {
	return `{
		"data": [
			{"id": 1789, "name": "Điện Tử - Điện Lạnh", "url_key": "dien-tu-dien-lanh", "children": []},
			{"id": 1815, "name": "Thiết Bị Số - Phụ Kiện Số", "url_key": "thiet-bi-so-phu-kien-so", "children": []},
			{"id": 8322, "name": "Đồ Gia Dụng", "url_key": "do-gia-dung", "children": []}
		]
	}`
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent header")
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q", body)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.APIBase = srv.URL + "/api/v2"
	cfg.Rate = 0
	cfg.Retries = 5
	cfg.Timeout = 5 * time.Second
	c := NewClientWithConfig(cfg)

	start := time.Now()
	_, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("hits = %d, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("no backoff between retries")
	}
}

func TestGetProduct(t *testing.T) {
	productJSON := sampleProductJSON(54538887, "iPhone 15 Pro Max")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(productJSON))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	p, err := c.GetProduct(context.Background(), "54538887")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "54538887" {
		t.Errorf("ID = %q, want 54538887", p.ID)
	}
	if p.Name != "iPhone 15 Pro Max" {
		t.Errorf("Name = %q, want iPhone 15 Pro Max", p.Name)
	}
	if p.Price != 150000 {
		t.Errorf("Price = %v, want 150000", p.Price)
	}
	if p.DiscountRate != 25 {
		t.Errorf("DiscountRate = %d, want 25", p.DiscountRate)
	}
	if p.BrandName != "TestBrand" {
		t.Errorf("BrandName = %q, want TestBrand", p.BrandName)
	}
	if !p.IsOfficialStore {
		t.Error("IsOfficialStore should be true")
	}
}

func TestListProducts(t *testing.T) {
	listJSON := sampleProductListJSON(5)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listJSON))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	products, err := c.ListProducts(context.Background(), "1789", "top_seller", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 5 {
		t.Fatalf("len = %d, want 5", len(products))
	}
	if products[0].Name != "Sản phẩm 1" {
		t.Errorf("first product name = %q", products[0].Name)
	}
}

func TestListProductsLimit(t *testing.T) {
	listJSON := sampleProductListJSON(10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listJSON))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	products, err := c.ListProducts(context.Background(), "", "", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 4 {
		t.Fatalf("len = %d, want 4", len(products))
	}
}

func TestListReviews(t *testing.T) {
	reviewJSON := sampleReviewListJSON(54538887, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reviewJSON))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	reviews, err := c.ListReviews(context.Background(), "54538887", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 3 {
		t.Fatalf("len = %d, want 3", len(reviews))
	}
	if reviews[0].ProductID != "54538887" {
		t.Errorf("ProductID = %q, want 54538887", reviews[0].ProductID)
	}
	if reviews[0].Title != "Đánh giá 1" {
		t.Errorf("Title = %q", reviews[0].Title)
	}
}

func TestListCategories(t *testing.T) {
	catJSON := sampleCategoryListJSON()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(catJSON))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	cats, err := c.ListCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 3 {
		t.Fatalf("len = %d, want 3", len(cats))
	}
	if cats[0].Name != "Điện Tử - Điện Lạnh" {
		t.Errorf("first cat name = %q", cats[0].Name)
	}
}

func TestProductURL(t *testing.T) {
	wire := wireProduct{
		ID:     54538887,
		URLKey: "iphone-15-pro-max",
		Name:   "iPhone 15",
	}
	p := productFromWire(wire, "https://tiki.vn")
	want := "https://tiki.vn/iphone-15-pro-max/p54538887.html"
	if p.URL != want {
		t.Errorf("URL = %q, want %q", p.URL, want)
	}
}

func TestGetHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Error("want error on 404")
	}
}
