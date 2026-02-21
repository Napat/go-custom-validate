package main

import (
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
)

type CreateProductRequest struct {
	Name      string `validate:"required"`
	ProductID string `validate:"required,product_id"` // เรียกใช้ Custom Tag ตรงนี้
}

func main() {
	v := validator.New()

	// ⚠️ Basic validator
	// but this may be slow code because it compiles regex every time (see main_test.go)
	v.RegisterValidation("product_id", func(fl validator.FieldLevel) bool {
		re := regexp.MustCompile(`^PROD-\d{4}$`)
		return re.MatchString(fl.Field().String())
	})

	req1 := CreateProductRequest{
		Name:      "Mechanical Keyboard",
		ProductID: "PROD-9999",
	}

	// Request1 should pass validation
	if err := v.Struct(req1); err != nil {
		fmt.Println("❌ Request1 Validation Failed:", err)
	} else {
		fmt.Println("✅ Request1 Validation Passed! 🎉")
	}

	req2 := CreateProductRequest{
		Name:      "Wireless Mouse",
		ProductID: "INVALID-123",
	}

	// Request2 should fail validation
	if err := v.Struct(req2); err != nil {
		fmt.Println("✅ Request2 Validation Failed:", err)
	} else {
		fmt.Println("❌ Request2 Validation Passed! 🎉")
	}
}
