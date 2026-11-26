package helpers

import (
	"html/template"
	"math"
	"net/url"
	"strconv"
)

// Pagination menyimpan metadata halaman untuk tampilan daftar data.
type Pagination struct {
	Page     int
	PerPage  int
	Total    int64
	LastPage int
	From     int
	To       int
	Pages    []int
	Query    url.Values
}

// Paginate menghitung batas halaman dan daftar nomor halaman berdasarkan total data.
func Paginate(page, perPage int, total int64) Pagination {
	if perPage < 1 {
		perPage = 10
	}
	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}
	if page < 1 {
		page = 1
	}
	if page > lastPage {
		page = lastPage
	}
	p := Pagination{Page: page, PerPage: perPage, Total: total, LastPage: lastPage}
	if total > 0 {
		p.From = (page-1)*perPage + 1
		p.To = min(page*perPage, int(total))
	}
	for i := 1; i <= lastPage; i++ {
		p.Pages = append(p.Pages, i)
	}
	return p
}

// Offset menghitung posisi awal baris untuk query database.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// HasPages memeriksa apakah jumlah halaman lebih dari satu.
func (p Pagination) HasPages() bool { return p.LastPage > 1 }

// HasPrev memeriksa apakah terdapat halaman sebelumnya.
func (p Pagination) HasPrev() bool { return p.Page > 1 }

// HasNext memeriksa apakah terdapat halaman berikutnya.
func (p Pagination) HasNext() bool { return p.Page < p.LastPage }

// PrevPage mengembalikan nomor halaman sebelumnya.
func (p Pagination) PrevPage() int { return p.Page - 1 }

// NextPage mengembalikan nomor halaman berikutnya.
func (p Pagination) NextPage() int { return p.Page + 1 }

// WithQuery menyertakan parameter query string agar filter tetap terbawa antar halaman.
func (p Pagination) WithQuery(params map[string]string) Pagination {
	p.Query = url.Values{}
	for k, v := range params {
		if v != "" && v != "0" {
			p.Query.Set(k, v)
		}
	}
	return p
}

// Link menyusun URL query string untuk nomor halaman tertentu.
func (p Pagination) Link(page int) template.URL {
	q := url.Values{}
	for k, v := range p.Query {
		q[k] = v
	}
	q.Set("page", strconv.Itoa(page))
	return template.URL("?" + q.Encode())
}

