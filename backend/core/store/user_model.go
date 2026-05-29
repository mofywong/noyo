package store

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user
type User struct {
	gorm.Model
	TenantID           uint       `gorm:"index;uniqueIndex:idx_tenant_username;not null;default:0" json:"tenant_id"`
	Username           string     `gorm:"uniqueIndex:idx_tenant_username;size:64;not null" json:"username"`
	Password           string     `gorm:"size:128;not null" json:"-"` // DO NOT return password in JSON
	DisplayName        string     `gorm:"size:64" json:"display_name"`
	Email              string     `gorm:"size:128" json:"email"`
	Role               string     `gorm:"size:32;not null;default:viewer" json:"role"` // admin, operator, viewer
	Status             int        `gorm:"default:1" json:"status"`                     // 1=active, 0=disabled
	MustChangePassword bool       `gorm:"default:false" json:"must_change_password"`
	LastLoginAt        *time.Time `json:"last_login_at"`
}

// GetUserByUsername retrieves a user by their username
func GetUserByUsername(username string) (*User, error) {
	var user User
	result := DB.Where("username = ?", username).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// GetUserByTenantAndUsername retrieves a user by tenant ID and username
func GetUserByTenantAndUsername(tenantID uint, username string) (*User, error) {
	var user User
	result := DB.Where("tenant_id = ? AND username = ?", tenantID, username).First(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// GetUsersByUsername retrieves all users by their username
func GetUsersByUsername(username string) ([]User, error) {
	var users []User
	result := DB.Where("username = ?", username).Find(&users)
	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

// GetUserByID retrieves a user by their ID
func GetUserByID(id uint) (*User, error) {
	var user User
	result := DB.First(&user, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

// SaveUser creates or updates a user
func SaveUser(u *User) error {
	if u.ID == 0 {
		return DB.Create(u).Error
	}
	return DB.Save(u).Error
}

// ListUsers retrieves users with pagination
func ListUsers(page, pageSize int, tenantID, projectID, roleID uint, isProjectAdmin bool, allowedProjectIDs []uint) ([]User, int64, error) {
	var users []User
	var total int64

	if isProjectAdmin && len(allowedProjectIDs) == 0 {
		return []User{}, 0, nil
	}

	db := DB.Model(&User{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}

	if isProjectAdmin {
		db = db.Where("id IN (?)", DB.Model(&UserRoleBinding{}).Select("user_id").Where("project_id IN ?", allowedProjectIDs))
	}

	if projectID > 0 {
		db = db.Where("id IN (?)", DB.Model(&UserRoleBinding{}).Select("user_id").Where("project_id = ?", projectID))
	}

	if roleID > 0 {
		db = db.Where("id IN (?)", DB.Model(&UserRoleBinding{}).Select("user_id").Where("role_id = ?", roleID))
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		db = db.Offset(offset).Limit(pageSize)
	}

	result := db.Order("created_at desc").Find(&users)
	return users, total, result.Error
}

// DeleteUser performs a soft delete on a user
func DeleteUser(id uint) error {
	return DB.Delete(&User{}, id).Error
}
