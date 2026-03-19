package seed

import (
	"common/pkg/auth"
	"common/pkg/permission"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
	"user-service/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var positions = []string{"Manager", "Developer", "HR"}

var employees = []struct {
	FirstName   string
	LastName    string
	Gender      string
	DateOfBirth string
	Email       string
	PhoneNumber string
	Address     string
	Username    string
	Password    string
	Active      bool
	Department  string
	Position    string
}{
	{"Dimitrije", "Mijailovic", "M", "1985-05-01", "dimitrije@raf.rs", "123456789", "Street 1", "dimitrije", "pass123", true, "IT", "Developer"},
	{"Petar", "Petrovic", "M", "1990-08-12", "petar@raf.rs", "987654321", "Street 2", "petar", "pass123", true, "HR", "HR"},
	{"Admin", "Admin", "M", "1980-01-01", "admin@raf.rs", "000000000", "RAF", "admin", "admin123", true, "IT", "Manager"},
}

var activatableClients = []struct {
	FirstName   string
	LastName    string
	Gender      string
	DateOfBirth string
	Email       string
	Username    string
	PhoneNumber string
	Address     string
	Password    string
}{
	{"Test", "Client", "M", "2000-01-01", "testclient@example.com", "testclient", "+381600000001", "Test Address 1, Beograd", "test123"},
}

var clients = []struct {
	FirstName   string
	LastName    string
	Gender      string
	DateOfBirth string
	Email       string
	Username    string
	PhoneNumber string
	Address     string
	Password    string
}{
	{"Marko", "Markovic", "M", "1992-03-15", "marko.markovic@example.com", "marko.markovic", "+381601234567", "Knez Mihailova 10, Beograd", "password123"},
	{"Ana", "Anic", "F", "1995-07-22", "ana.anic@example.com", "ana.anic", "+381609876543", "Bulevar Oslobodjenja 20, Novi Sad", "password123"},
	{"Stefan", "Stefanovic", "M", "1988-11-30", "stefan.stefanovic@example.com", "stefan.stefanovic", "+381611112222", "Trg Republike 5, Beograd", "password123"},
}

func Run(db *gorm.DB) error {
	// seed positions
	positionMap := make(map[string]uint)
	for _, title := range positions {
		var pos model.Position
		err := db.Where("title = ?", title).First(&pos).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			pos = model.Position{Title: title}
			if err := db.Create(&pos).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		positionMap[title] = pos.PositionID
	}

	// seed employees
	for _, e := range employees {
		var existingIdentity model.Identity
		if err := db.Where("email = ?", e.Email).First(&existingIdentity).Error; err == nil {
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(e.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		dob, err := time.Parse("2006-01-02", e.DateOfBirth)
		if err != nil {
			return err
		}

		identity := model.Identity{
			Email:        e.Email,
			Username:     e.Username,
			PasswordHash: string(hash),
			Type:         auth.IdentityEmployee,
			Active:       e.Active,
		}
		if err := db.Create(&identity).Error; err != nil {
			return err
		}

		employee := model.Employee{
			IdentityID:  identity.ID,
			FirstName:   e.FirstName,
			LastName:    e.LastName,
			Gender:      e.Gender,
			DateOfBirth: dob,
			PhoneNumber: e.PhoneNumber,
			Address:     e.Address,
			Department:  e.Department,
			PositionID:  positionMap[e.Position],
		}
		if err := db.Create(&employee).Error; err != nil {
			return err
		}
	}
	// seed clients
	for _, c := range clients {
		var existingIdentity model.Identity
		if err := db.Where("email = ?", c.Email).First(&existingIdentity).Error; err == nil {
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		dob, err := time.Parse("2006-01-02", c.DateOfBirth)
		if err != nil {
			return err
		}

		identity := model.Identity{
			Email:        c.Email,
			Username:     c.Username,
			PasswordHash: string(hash),
			Type:         auth.IdentityClient,
			Active:       true,
		}
		if err := db.Create(&identity).Error; err != nil {
			return err
		}

		client := model.Client{
			IdentityID:  identity.ID,
			FirstName:   c.FirstName,
			LastName:    c.LastName,
			Gender:      c.Gender,
			DateOfBirth: dob,
			PhoneNumber: c.PhoneNumber,
			Address:     c.Address,
		}
		if err := db.Create(&client).Error; err != nil {
			return err
		}
	}
	// seed activatable clients, activated in place
	for _, c := range activatableClients {
		var existingIdentity model.Identity
		if err := db.Where("email = ?", c.Email).First(&existingIdentity).Error; err == nil {
			continue // already seeded
		}

		dob, err := time.Parse("2006-01-02", c.DateOfBirth)
		if err != nil {
			return err
		}

		identity := model.Identity{
			Email:    c.Email,
			Username: c.Username,
			Type:     auth.IdentityClient,
			Active:   false,
		}
		if err := db.Create(&identity).Error; err != nil {
			return err
		}

		client := model.Client{
			IdentityID:  identity.ID,
			FirstName:   c.FirstName,
			LastName:    c.LastName,
			Gender:      c.Gender,
			DateOfBirth: dob,
			PhoneNumber: c.PhoneNumber,
			Address:     c.Address,
		}
		if err := db.Create(&client).Error; err != nil {
			return err
		}

		// generate and insert an activation token
		rawBytes := make([]byte, 16)
		if _, err := rand.Read(rawBytes); err != nil {
			return err
		}
		tokenStr := hex.EncodeToString(rawBytes)

		activationToken := model.ActivationToken{
			IdentityID: identity.ID,
			Token:      tokenStr,
			ExpiresAt:  time.Now().Add(24 * time.Hour),
		}
		if err := db.Create(&activationToken).Error; err != nil {
			return err
		}

		// activate in-place
		hash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Model(&identity).Updates(map[string]any{
			"password_hash": string(hash),
			"active":        true,
		}).Error; err != nil {
			return err
		}
		if err := db.Delete(&activationToken).Error; err != nil {
			return err
		}
	}

	// seed admin permissions
	var adminIdentity model.Identity
	if err := db.Where("email = ?", "admin@raf.rs").First(&adminIdentity).Error; err != nil {
		return err
	}

	var admin model.Employee
	if err := db.Where("identity_id = ?", adminIdentity.ID).First(&admin).Error; err != nil {
		return err
	}

	for _, p := range permission.All {
		var existing model.EmployeePermission
		err := db.Where("employee_id = ? AND permission = ?", admin.EmployeeID, string(p)).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			perm := model.EmployeePermission{
				EmployeeID: admin.EmployeeID,
				Permission: p,
			}
			if err := db.Create(&perm).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}
