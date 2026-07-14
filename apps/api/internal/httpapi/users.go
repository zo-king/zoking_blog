package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zo-king/zoking_blog/apps/api/internal/auth"
	"github.com/zo-king/zoking_blog/apps/api/internal/model"
	"gorm.io/gorm"
)

type adminUserResponse struct {
	ID          uuid.UUID   `json:"id"`
	Email       string      `json:"email"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	Status      string      `json:"status"`
	Roles       []string    `json:"roles"`
	CreatedAt   interface{} `json:"created_at"`
}

type roleResponse struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
}

func listAdminUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pagination, ok := parsePagination(c)
		if !ok {
			return
		}
		order, ok := parseListOrder(c, pagination.Sort, map[string]string{
			"created_at":   "created_at",
			"email":        "email",
			"username":     "username",
			"display_name": "display_name",
			"status":       "status",
		})
		if !ok {
			return
		}

		query := db.WithContext(c.Request.Context()).Model(&model.User{})
		if pagination.Query != "" {
			pattern := "%" + pagination.Query + "%"
			query = query.Where("email ILIKE ? OR username ILIKE ? OR display_name ILIKE ?", pattern, pattern, pattern)
		}
		if pagination.Status != "" {
			query = query.Where("status = ?", pagination.Status)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not count users")
			return
		}
		if returnEmptyPageIfOutOfRange[adminUserResponse](c, total, pagination) {
			return
		}

		var users []model.User
		if err := query.Order(order + ", id ASC").Offset(pagination.Offset).Limit(pagination.PageSize).Find(&users).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list users")
			return
		}
		result := make([]adminUserResponse, 0, len(users))
		for _, user := range users {
			roles, err := roleCodesForUser(c, db, user.ID)
			if err != nil {
				Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not load user roles")
				return
			}
			result = append(result, adminUserResponse{ID: user.ID, Email: user.Email, Username: user.Username, DisplayName: user.DisplayName, Status: user.Status, Roles: roles, CreatedAt: user.CreatedAt})
		}
		OKPaginated(c, result, total, pagination)
	}
}

func parseListOrder(c *gin.Context, value string, allowed map[string]string) (string, bool) {
	if value == "" {
		value = "-created_at"
	}

	parts := strings.Split(value, ",")
	orders := make([]string, 0, len(parts))
	for _, part := range parts {
		direction := "ASC"
		field := part
		if strings.HasPrefix(field, "-") {
			direction = "DESC"
			field = strings.TrimPrefix(field, "-")
		}
		column, ok := allowed[field]
		if !ok || field == "" {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid sort")
			return "", false
		}
		orders = append(orders, column+" "+direction)
	}
	return strings.Join(orders, ", "), true
}

func createAdminUser(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Email       string   `json:"email" binding:"required,email"`
		Username    string   `json:"username" binding:"required,min=3,max=64"`
		DisplayName string   `json:"display_name" binding:"max=120"`
		Password    string   `json:"password" binding:"required,min=10,max=128"`
		RoleCodes   []string `json:"role_codes" binding:"required,min=1"`
	}
	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid user payload")
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "could not secure password")
			return
		}
		user := model.User{Email: strings.TrimSpace(req.Email), Username: strings.TrimSpace(req.Username), DisplayName: strings.TrimSpace(req.DisplayName), PasswordHash: hash, Status: "active"}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			return replaceUserRoles(tx, user.ID, req.RoleCodes)
		})
		if err != nil {
			Fail(c, http.StatusConflict, "USER_CREATE_FAILED", "email, username, or roles are invalid")
			return
		}
		roles, _ := roleCodesForUser(c, db, user.ID)
		Created(c, adminUserResponse{ID: user.ID, Email: user.Email, Username: user.Username, DisplayName: user.DisplayName, Status: user.Status, Roles: roles, CreatedAt: user.CreatedAt})
	}
}

func updateAdminUserStatus(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Status string `json:"status" binding:"required,oneof=active disabled"`
	}
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid user id")
			return
		}
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid status")
			return
		}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := lockSuperAdminGuard(tx); err != nil {
				return err
			}
			if req.Status != "active" {
				protected, err := isLastActiveSuperAdmin(tx, userID)
				if err != nil {
					return err
				}
				if protected {
					return errLastSuperAdmin
				}
			}
			result := tx.Model(&model.User{}).Where("id = ? and deleted_at is null", userID).Update("status", req.Status)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			return nil
		})
		if errors.Is(err, errLastSuperAdmin) {
			Fail(c, http.StatusConflict, "LAST_SUPER_ADMIN", "the last active super administrator cannot be disabled")
			return
		}
		if err != nil {
			Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
			return
		}
		OK(c, gin.H{"id": userID, "status": req.Status})
	}
}

func updateAdminUserRoles(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		RoleCodes []string `json:"role_codes" binding:"required,min=1"`
	}
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid user id")
			return
		}
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid roles")
			return
		}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			if err := lockSuperAdminGuard(tx); err != nil {
				return err
			}
			removesSuper := true
			for _, code := range req.RoleCodes {
				if code == "super_admin" {
					removesSuper = false
				}
			}
			if removesSuper {
				protected, err := isLastActiveSuperAdmin(tx, userID)
				if err != nil {
					return err
				}
				if protected {
					return errLastSuperAdmin
				}
			}
			return replaceUserRoles(tx, userID, req.RoleCodes)
		})
		if errors.Is(err, errLastSuperAdmin) {
			Fail(c, http.StatusConflict, "LAST_SUPER_ADMIN", "the last active super administrator cannot lose that role")
			return
		}
		if err != nil {
			Fail(c, http.StatusConflict, "ROLE_ASSIGNMENT_FAILED", "roles are invalid")
			return
		}
		roles, _ := roleCodesForUser(c, db, userID)
		OK(c, gin.H{"id": userID, "roles": roles})
	}
}

func resetAdminUserPassword(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Password string `json:"password" binding:"required,min=10,max=72"`
	}
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, 422, "VALIDATION_FAILED", "invalid user id")
			return
		}
		var req request
		if err := c.ShouldBindJSON(&req); err != nil || len([]byte(req.Password)) > 72 {
			Fail(c, 422, "VALIDATION_FAILED", "password must be between 10 and 72 UTF-8 bytes")
			return
		}
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			Fail(c, 500, "PASSWORD_HASH_FAILED", "could not secure password")
			return
		}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&model.User{}).Where("id=? and deleted_at is null", userID).Update("password_hash", hash)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			return tx.Exec("delete from refresh_tokens where user_id=?", userID).Error
		})
		if err != nil {
			Fail(c, 404, "USER_NOT_FOUND", "user not found")
			return
		}
		OK(c, gin.H{"id": userID, "password_reset": true})
	}
}

func listAdminRoles(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rows []struct {
			ID                      uuid.UUID
			Code, Name, Description string
			IsSystem                bool
		}
		if err := db.WithContext(c.Request.Context()).Table("roles").Order("code").Scan(&rows).Error; err != nil {
			Fail(c, 500, "INTERNAL_ERROR", "could not list roles")
			return
		}
		result := make([]roleResponse, 0, len(rows))
		for _, row := range rows {
			var permissions []string
			if err := db.WithContext(c.Request.Context()).Table("permissions p").Select("p.code").Joins("join role_permissions rp on rp.permission_id=p.id").Where("rp.role_id=?", row.ID).Order("p.code").Pluck("p.code", &permissions).Error; err != nil {
				Fail(c, 500, "INTERNAL_ERROR", "could not load role permissions")
				return
			}
			result = append(result, roleResponse{ID: row.ID, Code: row.Code, Name: row.Name, Description: row.Description, IsSystem: row.IsSystem, Permissions: permissions})
		}
		OK(c, result)
	}
}

var errLastSuperAdmin = errors.New("last active super administrator")

func lockSuperAdminGuard(tx *gorm.DB) error {
	return tx.Exec("select pg_advisory_xact_lock(hashtext('zoking:last-super-admin'))").Error
}

func isLastActiveSuperAdmin(tx *gorm.DB, target uuid.UUID) (bool, error) {
	var targetIsSuper int64
	if err := tx.Table("user_roles ur").Joins("join roles r on r.id=ur.role_id").Where("ur.user_id=? and r.code='super_admin'", target).Count(&targetIsSuper).Error; err != nil {
		return false, err
	}
	if targetIsSuper == 0 {
		return false, nil
	}
	var count int64
	err := tx.Table("users u").Joins("join user_roles ur on ur.user_id=u.id").Joins("join roles r on r.id=ur.role_id").Where("u.status='active' and u.deleted_at is null and r.code='super_admin'").Distinct("u.id").Count(&count).Error
	return count <= 1, err
}

func replaceUserRoles(tx *gorm.DB, userID uuid.UUID, codes []string) error {
	var count int64
	if err := tx.Table("roles").Where("code in ?", codes).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(uniqueStrings(codes))) {
		return errors.New("unknown role")
	}
	if err := tx.Exec("delete from user_roles where user_id=?", userID).Error; err != nil {
		return err
	}
	return tx.Exec("insert into user_roles(user_id,role_id) select ?,id from roles where code in ?", userID, uniqueStrings(codes)).Error
}

func roleCodesForUser(c *gin.Context, db *gorm.DB, userID uuid.UUID) ([]string, error) {
	var roles []string
	err := db.WithContext(c.Request.Context()).Table("roles r").Select("r.code").Joins("join user_roles ur on ur.role_id=r.id").Where("ur.user_id=?", userID).Order("r.code").Pluck("r.code", &roles).Error
	return roles, err
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
