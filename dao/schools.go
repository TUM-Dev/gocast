package dao

import (
	"context"

	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=schools.go -destination ../mock_dao/schools.go

type SchoolsDao interface {
	// Get School by ID
	Get(context.Context, uint) (model.School, error)

	// Create a new School for the database
	Create(context.Context, *model.School) error

	// Delete a School by id.
	Delete(context.Context, uint) error

	// Update a School
	Update(context.Context, *model.School) error

	// Search for Schools by query
	QueryAdministerdSchools(context.Context, *model.User, string) ([]model.School, error)

	// Get all Admins for a School
	GetAdmins(context.Context, uint) ([]model.User, error)

	// Add an Admin to a School
	AddAdmin(context.Context, *model.School, *model.User) error

	// Remove an Admin from a School
	RemoveAdmin(context.Context, uint, uint) error

	// Get all Schools administered by a User
	GetAdministeredSchoolsByUserId(context.Context, uint) ([]model.School, error)

	// Get by name and university
	GetByNameAndUniversity(context.Context, string, string) (model.School, error)

	// Get admin count
	GetAdminCount(context.Context, uint) (int, error)

	// Get all Admins for a School
	GetAdminsBySchoolAndUniversity(context.Context, string, string) ([]model.User, error)

	// Get all Resources for a School
	// GetResources(context.Context, uint) ([]model.Resource, error)

	// Add a Resource to a School
	// AddResource(context.Context, uint, uint) error

	// Remove a Resource from a School
	// RemoveResource(context.Context, uint, uint) error

	// Get all Resources for a School
	// GetResources(context.Context, uint) ([]model.Resource, error)
}

type schoolDao struct {
	db *gorm.DB
}

func NewSchoolsDao() SchoolsDao {
	return schoolDao{db: DB}
}

// Get a School by id.
func (d schoolDao) Get(c context.Context, id uint) (res model.School, err error) {
	return res, d.db.WithContext(c).Preload("Admins").First(&res, id).Error
}

// Create a School and init super-admins.
func (d schoolDao) Create(c context.Context, it *model.School) error {
	if err := d.db.WithContext(c).Create(it).Error; err != nil {
		return err
	}

	// Get all admins of the 'master' school of the 'service' university
	admins, err := d.GetAdminsBySchoolAndUniversity(c, "master", "service")
	if err != nil {
		return err
	}

	// Add each admin to the new school
	for _, admin := range admins {
		if err := d.AddAdmin(c, it, &admin); err != nil {
			return err
		}
	}

	return nil
}

// Delete a School by id.
func (d schoolDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.School{}, id).Error
}

func (d schoolDao) Update(c context.Context, it *model.School) error {
	return d.db.WithContext(c).Model(it).Updates(it).Error
}

func (d schoolDao) QueryAdministerdSchools(c context.Context, admin *model.User, query string) (res []model.School, err error) {
	return res, d.db.WithContext(c).
		Joins("JOIN school_admins ON school_admins.school_id = schools.id").
		Preload("Admins").
		Where("(schools.name LIKE ? OR schools.university LIKE ?) AND school_admins.user_id = ?", "%"+query+"%", "%"+query+"%", admin.ID).
		Find(&res).Error
}

func (d schoolDao) GetAdmins(c context.Context, id uint) (res []model.User, err error) {
	return res, d.db.WithContext(c).Model(&model.School{Model: gorm.Model{ID: id}}).Association("Admins").Find(&res)
}

func (d schoolDao) AddAdmin(c context.Context, school *model.School, admin *model.User) error {
	logger.Info("Adding Admin to School", "adminID", admin)
	return d.db.WithContext(c).Model(school).Association("Admins").Append(admin)
}

func (d schoolDao) RemoveAdmin(c context.Context, schoolID, adminID uint) error {
	return d.db.WithContext(c).Model(&model.School{Model: gorm.Model{ID: schoolID}}).Association("Admins").Delete(&model.User{Model: gorm.Model{ID: adminID}})
}

func (d schoolDao) GetAdministeredSchoolsByUserId(c context.Context, userID uint) (res []model.School, err error) {
	return res, d.db.WithContext(c).Preload("Admins").Model(&model.User{Model: gorm.Model{ID: userID}}).Association("AdministeredSchools").Find(&res)
}

func (d schoolDao) GetByNameAndUniversity(c context.Context, name, university string) (res model.School, err error) {
	return res, d.db.WithContext(c).Where("name = ? AND university = ?", name, university).First(&res).Error
}

func (d schoolDao) GetAdminCount(c context.Context, id uint) (int, error) {
	var school model.School
	if err := d.db.WithContext(c).First(&school, id).Error; err != nil {
		return 0, err
	}
	count := d.db.Model(&school).Association("Admins").Count()
	return int(count), nil
}

func (d schoolDao) GetAdminsBySchoolAndUniversity(c context.Context, name, university string) (res []model.User, err error) {
	return res, d.db.WithContext(c).
		Joins("JOIN school_admins ON school_admins.user_id = users.id").
		Joins("JOIN schools ON schools.id = school_admins.school_id").
		Where("schools.name = ? AND schools.university = ?", name, university).
		Find(&res).Error
}

/* TODO: For later use when resources are implemented:
 func (d schoolDao) GetResources(c context.Context, id uint) (res []model.Resource, err error) {
	return res, d.db.WithContext(c).Model(&model.School{Model: gorm.Model{ID: id}}).Association("Resources").Find(&res)
}

func (d schoolDao) AddResource(c context.Context, schoolID, resourceID uint) error {
	return d.db.WithContext(c).Model(&model.School{Model: gorm.Model{ID: schoolID}}).Association("Resources").Append(&model.Resource{Model: gorm.Model{ID: resourceID}})
}

func (d schoolDao) RemoveResource(c context.Context, schoolID, resourceID uint) error {
	return d.db.WithContext(c).Model(&model.School{Model: gorm.Model{ID: schoolID}}).Association("Resources").Delete(&model.Resource{Model: gorm.Model{ID: resourceID}})
}
*/
