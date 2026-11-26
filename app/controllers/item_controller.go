package controllers

import (
	"fmt"
	"strings"

	"checkweigher/app/helpers"
	"checkweigher/app/models"
	"checkweigher/app/requests"
	"checkweigher/config"

	"github.com/gofiber/fiber/v3"
)

type ItemController struct{}

// Index menampilkan daftar barang terdaftar dengan fitur pencarian dan pagination.
func (ctl *ItemController) Index(c fiber.Ctx) error {
	search := strings.TrimSpace(c.Query("q"))
	page := fiber.Query[int](c, "page", 1)

	query := config.DB.Model(&models.Item{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("(name LIKE ? OR item_code LIKE ? OR rfid_uid LIKE ?)", like, like, like)
	}

	var total int64
	query.Count(&total)

	pagination := helpers.Paginate(page, 10, total).WithQuery(map[string]string{"q": search})

	var items []models.Item
	query.Order("id desc").
		Limit(pagination.PerPage).
		Offset(pagination.Offset()).
		Find(&items)

	return helpers.View(c, "items/index", fiber.Map{
		"Title":      "Daftar Barang",
		"Items":      items,
		"Pagination": pagination,
		"Search":     search,
	})
}

// Create menampilkan form pendaftaran barang dan tag RFID baru.
func (ctl *ItemController) Create(c fiber.Ctx) error {
	return helpers.View(c, "items/create", fiber.Map{
		"Title": "Daftarkan Barang Baru",
		"Form":  requests.ItemRequest{},
	})
}

// Store memvalidasi dan menyimpan data barang baru beserta kode barang otomatis.
func (ctl *ItemController) Store(c fiber.Ctx) error {
	var req requests.ItemRequest
	if err := c.Bind().Form(&req); err != nil {
		return c.Redirect().With("error", "Format input tidak valid.").Back("/items/create")
	}

	req.RfidUID = strings.ToUpper(strings.TrimSpace(req.RfidUID))
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	errs := requests.Validate(req)

	var existingCount int64
	config.DB.Model(&models.Item{}).Where("rfid_uid = ?", req.RfidUID).Count(&existingCount)
	if existingCount > 0 {
		errs["rfid_uid"] = "RFID UID ini sudah terdaftar untuk barang lain."
	}

	if len(errs) > 0 {
		c.Status(fiber.StatusUnprocessableEntity)
		return helpers.View(c, "items/create", fiber.Map{
			"Title":  "Daftarkan Barang Baru",
			"Form":   req,
			"Errors": errs,
		})
	}

	var maxID uint
	config.DB.Model(&models.Item{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID)
	nextCode := fmt.Sprintf("BRG-%02d", maxID+1)

	item := models.Item{
		ItemCode:    nextCode,
		RfidUID:     req.RfidUID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := config.DB.Create(&item).Error; err != nil {
		return c.Redirect().With("error", "Gagal menyimpan barang: "+err.Error()).To("/items")
	}

	return c.Redirect().With("success", "Barang "+item.Name+" ("+item.ItemCode+") berhasil didaftarkan.").To("/items")
}

// Edit menampilkan form ubah data barang berdasarkan ID.
func (ctl *ItemController) Edit(c fiber.Ctx) error {
	var item models.Item
	if err := config.DB.First(&item, c.Params("id")).Error; err != nil {
		return fiber.ErrNotFound
	}

	return helpers.View(c, "items/edit", fiber.Map{
		"Title": "Ubah Data Barang",
		"Item":  item,
		"Form": requests.ItemRequest{
			RfidUID:     item.RfidUID,
			Name:        item.Name,
			Description: item.Description,
		},
	})
}

// Update memvalidasi dan memperbarui data barang yang dipilih.
func (ctl *ItemController) Update(c fiber.Ctx) error {
	var item models.Item
	if err := config.DB.First(&item, c.Params("id")).Error; err != nil {
		return fiber.ErrNotFound
	}

	var req requests.ItemRequest
	if err := c.Bind().Form(&req); err != nil {
		return c.Redirect().With("error", "Format input tidak valid.").Back("/items")
	}

	req.RfidUID = strings.ToUpper(strings.TrimSpace(req.RfidUID))
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	errs := requests.Validate(req)

	var count int64
	config.DB.Model(&models.Item{}).Where("rfid_uid = ? AND id <> ?", req.RfidUID, item.ID).Count(&count)
	if count > 0 {
		errs["rfid_uid"] = "RFID UID ini sudah terdaftar untuk barang lain."
	}

	if len(errs) > 0 {
		c.Status(fiber.StatusUnprocessableEntity)
		return helpers.View(c, "items/edit", fiber.Map{
			"Title":  "Ubah Data Barang",
			"Item":   item,
			"Form":   req,
			"Errors": errs,
		})
	}

	item.RfidUID = req.RfidUID
	item.Name = req.Name
	item.Description = req.Description

	config.DB.Save(&item)
	return c.Redirect().With("success", "Data barang "+item.ItemCode+" berhasil diperbarui.").To("/items")
}

// Destroy menghapus data barang berdasarkan ID.
func (ctl *ItemController) Destroy(c fiber.Ctx) error {
	var item models.Item
	if err := config.DB.First(&item, c.Params("id")).Error; err != nil {
		return fiber.ErrNotFound
	}

	config.DB.Delete(&item)
	return c.Redirect().With("success", "Barang "+item.Name+" ("+item.ItemCode+") berhasil dihapus.").To("/items")
}

