package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func listAdminPermissions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var permissions []string
		if err := db.WithContext(c.Request.Context()).Table("permissions").Order("code").Pluck("code", &permissions).Error; err != nil {
			Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "could not list permissions")
			return
		}
		OK(c, permissions)
	}
}

func createAdminRole(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Code            string   `json:"code" binding:"required"`
		Name            string   `json:"name" binding:"required,max=120"`
		Description     string   `json:"description" binding:"max=500"`
		PermissionCodes []string `json:"permission_codes"`
	}
	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "invalid role payload")
			return
		}
		req.Code = strings.ToLower(strings.TrimSpace(req.Code))
		if !roleCodePattern.MatchString(req.Code) {
			Fail(c, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "role code must use lowercase letters, numbers, and underscores")
			return
		}
		var roleID uuid.UUID
		err := db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var roleIDText string
			if err := tx.Raw("insert into roles(code,name,description,is_system) values(?,?,?,false) returning id", req.Code, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description)).Scan(&roleIDText).Error; err != nil {
				return err
			}
			parsedRoleID, err := uuid.Parse(roleIDText)
			if err != nil {
				return err
			}
			roleID = parsedRoleID
			return replaceRolePermissions(tx, roleID, req.PermissionCodes)
		})
		if err != nil {
			Fail(c, http.StatusConflict, "ROLE_CREATE_FAILED", "role code or permissions are invalid")
			return
		}
		role, err := loadRoleResponse(c, db, roleID)
		if err != nil {
			Fail(c, 500, "INTERNAL_ERROR", "could not load role")
			return
		}
		Created(c, role)
	}
}

func updateAdminRole(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		Name        string `json:"name" binding:"required,max=120"`
		Description string `json:"description" binding:"max=500"`
	}
	return func(c *gin.Context) {
		roleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, 422, "VALIDATION_FAILED", "invalid role id")
			return
		}
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, 422, "VALIDATION_FAILED", "invalid role payload")
			return
		}
		result := db.WithContext(c.Request.Context()).Table("roles").Where("id=? and is_system=false", roleID).Updates(map[string]interface{}{"name": strings.TrimSpace(req.Name), "description": strings.TrimSpace(req.Description)})
		if result.Error != nil {
			Fail(c, 409, "ROLE_UPDATE_FAILED", "could not update role")
			return
		}
		if result.RowsAffected != 1 {
			Fail(c, 409, "SYSTEM_ROLE_PROTECTED", "system roles cannot be modified")
			return
		}
		role, _ := loadRoleResponse(c, db, roleID)
		OK(c, role)
	}
}

func updateAdminRolePermissions(db *gorm.DB) gin.HandlerFunc {
	type request struct {
		PermissionCodes []string `json:"permission_codes"`
	}
	return func(c *gin.Context) {
		roleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, 422, "VALIDATION_FAILED", "invalid role id")
			return
		}
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, 422, "VALIDATION_FAILED", "invalid permissions")
			return
		}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var isSystem bool
			if err := tx.Table("roles").Select("is_system").Where("id=?", roleID).Scan(&isSystem).Error; err != nil {
				return err
			}
			if isSystem {
				return errSystemRole
			}
			return replaceRolePermissions(tx, roleID, req.PermissionCodes)
		})
		if errors.Is(err, errSystemRole) {
			Fail(c, 409, "SYSTEM_ROLE_PROTECTED", "system role permissions cannot be modified")
			return
		}
		if err != nil {
			Fail(c, 409, "PERMISSION_ASSIGNMENT_FAILED", "permissions are invalid")
			return
		}
		role, _ := loadRoleResponse(c, db, roleID)
		OK(c, role)
	}
}

func deleteAdminRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Fail(c, 422, "VALIDATION_FAILED", "invalid role id")
			return
		}
		err = db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var isSystem bool
			if err := tx.Table("roles").Select("is_system").Where("id=?", roleID).Scan(&isSystem).Error; err != nil {
				return err
			}
			if isSystem {
				return errSystemRole
			}
			var refs int64
			if err := tx.Table("user_roles").Where("role_id=?", roleID).Count(&refs).Error; err != nil {
				return err
			}
			if refs > 0 {
				return errRoleInUse
			}
			return tx.Exec("delete from roles where id=? and is_system=false", roleID).Error
		})
		if errors.Is(err, errSystemRole) {
			Fail(c, 409, "SYSTEM_ROLE_PROTECTED", "system roles cannot be deleted")
			return
		}
		if errors.Is(err, errRoleInUse) {
			Fail(c, 409, "ROLE_IN_USE", "role is assigned to users")
			return
		}
		if err != nil {
			Fail(c, 404, "ROLE_NOT_FOUND", "role not found")
			return
		}
		OK(c, gin.H{"deleted": true, "id": roleID})
	}
}

var errSystemRole = errors.New("system role protected")
var errRoleInUse = errors.New("role in use")

func replaceRolePermissions(tx *gorm.DB, roleID uuid.UUID, codes []string) error {
	codes = uniqueStrings(codes)
	var count int64
	if len(codes) > 0 {
		if err := tx.Table("permissions").Where("code in ?", codes).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(codes)) {
			return errors.New("unknown permission")
		}
	}
	if err := tx.Exec("delete from role_permissions where role_id=?", roleID).Error; err != nil {
		return err
	}
	if len(codes) == 0 {
		return nil
	}
	return tx.Exec("insert into role_permissions(role_id,permission_id) select ?,id from permissions where code in ?", roleID, codes).Error
}

func loadRoleResponse(c *gin.Context, db *gorm.DB, roleID uuid.UUID) (roleResponse, error) {
	var role roleResponse
	if err := db.WithContext(c.Request.Context()).Table("roles").Where("id=?", roleID).Scan(&role).Error; err != nil {
		return role, err
	}
	if err := db.WithContext(c.Request.Context()).Table("permissions p").Select("p.code").Joins("join role_permissions rp on rp.permission_id=p.id").Where("rp.role_id=?", roleID).Order("p.code").Pluck("p.code", &role.Permissions).Error; err != nil {
		return role, err
	}
	return role, nil
}
