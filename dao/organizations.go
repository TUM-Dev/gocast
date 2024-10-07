package dao

import (
	"context"

	"github.com/TUM-Dev/gocast/model"
	"gorm.io/gorm"
)

//go:generate mockgen -source=organizations.go -destination ../mock_dao/organizations.go

type OrganizationsDao interface {
	/* ==> ORGANIZATION FUNCTIONS <== */

	// Get Organization by ID
	Get(context.Context, uint) (model.Organization, error)

	// Get all Organizations
	GetAll() []model.Organization

	// Returns all Organizations that match the query
	Query(context.Context, string) ([]model.Organization, error)

	// Search for Organizations by query
	QueryAdministerdOrganizations(context.Context, *model.User, string) ([]model.Organization, error)

	// Get all Organizations administered by a User
	GetAdministeredOrganizationsByUser(context.Context, *model.User) ([]model.Organization, error)

	// Get by name
	GetByName(context.Context, string) (model.Organization, error)

	// Create a new Organization for the database
	Create(context.Context, *model.Organization) error

	// Delete a Organization by id.
	Delete(context.Context, uint) error

	// Update a Organization
	Update(context.Context, *model.Organization) error

	/* ==> MAINTAINER FUNCTIONS <== */

	// Get all Admins for a Organization
	GetAdmins(context.Context, uint) ([]model.User, error)

	// Add an Admin to a Organization
	AddAdmin(context.Context, *model.Organization, *model.User) error

	// Remove an Admin from a Organization
	RemoveAdmin(context.Context, uint, uint) error

	// Get admin count
	GetAdminCount(context.Context, uint) (int, error)

	/* ==> CRON JOBS <== */
	ImportOrganization(string, string, string, string)

	/* ==> RESOURCE FUNCTIONS <== */

	// Get all Resources for a Organization
	// GetResources(context.Context, uint) ([]model.Resource, error)

	// Add a Resource to a Organization
	// AddResource(context.Context, uint, uint) error

	// Remove a Resource from a Organization
	// RemoveResource(context.Context, uint, uint) error

	// Get all Resources for a Organization
	// GetResources(context.Context, uint) ([]model.Resource, error)
}

type organizationDao struct {
	db *gorm.DB
}

func NewOrganizationsDao() OrganizationsDao {
	return organizationDao{db: DB}
}

// Get a Organization by id.
func (d organizationDao) Get(c context.Context, id uint) (res model.Organization, err error) {
	return res, d.db.WithContext(c).Preload("Admins").First(&res, id).Error
}

// Get all Organizations.
func (d organizationDao) GetAll() (res []model.Organization) {
	d.db.Find(&res)
	return res
}

// Create a Organization and init super-admins.
func (d organizationDao) Create(c context.Context, it *model.Organization) error {
	if err := d.db.WithContext(c).Create(it).Error; err != nil {
		return err
	}

	// Add each admin to the new organization
	admins := []model.User{}
	d.db.WithContext(c).Where("role = ?", model.AdminType).Find(&admins)
	for _, admin := range admins {
		if err := d.AddAdmin(c, it, &admin); err != nil {
			return err
		}
	}

	return nil
}

// Delete a Organization by id.
func (d organizationDao) Delete(c context.Context, id uint) error {
	return d.db.WithContext(c).Delete(&model.Organization{}, id).Error
}

func (d organizationDao) Update(c context.Context, it *model.Organization) error {
	return d.db.WithContext(c).Model(it).Updates(it).Error
}

func (d organizationDao) QueryAdministerdOrganizations(c context.Context, user *model.User, query string) (res []model.Organization, err error) {
	if user.Role == model.AdminType {
		return res, d.db.WithContext(c).Where("name LIKE ? OR org_type LIKE ?", "%"+query+"%", "%"+query+"%").Find(&res).Error
	} else {
		return res, d.db.WithContext(c).
			Joins("JOIN organization_admins ON organization_admins.organization_id = organizations.id").
			Preload("Admins").
			Where("(organizations.name LIKE ? OR organizations.org_type LIKE ?) AND organization_admins.user_id = ?", "%"+query+"%", "%"+query+"%", user.ID).
			Find(&res).Error
	}
}

func (d organizationDao) Query(c context.Context, query string) (res []model.Organization, err error) {
	return res, d.db.WithContext(c).Where("name LIKE ? OR org_type LIKE ?", "%"+query+"%", "%"+query+"%").Find(&res).Error
}

func (d organizationDao) GetAdmins(c context.Context, id uint) (res []model.User, err error) {
	return res, d.db.WithContext(c).Model(&model.Organization{Model: gorm.Model{ID: id}}).Association("Admins").Find(&res)
}

func (d organizationDao) AddAdmin(c context.Context, organization *model.Organization, admin *model.User) error {
	return d.db.WithContext(c).Model(organization).Association("Admins").Append(admin)
}

func (d organizationDao) RemoveAdmin(c context.Context, organizationID, adminID uint) error {
	return d.db.WithContext(c).Model(&model.Organization{Model: gorm.Model{ID: organizationID}}).Association("Admins").Delete(&model.User{Model: gorm.Model{ID: adminID}})
}

func (d organizationDao) GetAdministeredOrganizationsByUser(c context.Context, user *model.User) (res []model.Organization, err error) {
	if user.Role == model.AdminType {
		return res, d.db.WithContext(c).Preload("Admins").Preload("Workers").Preload("Runners").Preload("IngestServers").Find(&res).Error
	} else {
		return res, d.db.WithContext(c).Preload("Admins").Preload("Workers").Preload("Runners").Preload("IngestServers").Model(&model.User{Model: gorm.Model{ID: user.ID}}).Association("AdministeredOrganizations").Find(&res)
	}
}

func (d organizationDao) GetByName(c context.Context, name string) (res model.Organization, err error) {
	return res, d.db.WithContext(c).Where("name = ?", name).First(&res).Error
}

func (d organizationDao) GetAdminCount(c context.Context, id uint) (int, error) {
	var organization model.Organization
	if err := d.db.WithContext(c).First(&organization, id).Error; err != nil {
		return 0, err
	}
	count := d.db.Model(&organization).Association("Admins").Count()
	return int(count), nil
}

/* TODO: For later use when resources are implemented:
 func (d organizationDao) GetResources(c context.Context, id uint) (res []model.Resource, err error) {
	return res, d.db.WithContext(c).Model(&model.Organization{Model: gorm.Model{ID: id}}).Association("Resources").Find(&res)
}

func (d organizationDao) AddResource(c context.Context, organizationID, resourceID uint) error {
	return d.db.WithContext(c).Model(&model.Organization{Model: gorm.Model{ID: organizationID}}).Association("Resources").Append(&model.Resource{Model: gorm.Model{ID: resourceID}})
}

func (d organizationDao) RemoveResource(c context.Context, organizationID, resourceID uint) error {
	return d.db.WithContext(c).Model(&model.Organization{Model: gorm.Model{ID: organizationID}}).Association("Resources").Delete(&model.Resource{Model: gorm.Model{ID: resourceID}})
}
*/

func (d organizationDao) ImportOrganization(nr, kennung, orgTypName, nameEn string) {
	organization := model.Organization{}
	d.db.FirstOrCreate(&organization, model.Organization{
		OrgId:   nr,
		OrgSlug: kennung,
		OrgType: orgTypName,
	})

	organization.Name = nameEn

	d.db.Save(&organization)
	d.db.Create(organization)
}
