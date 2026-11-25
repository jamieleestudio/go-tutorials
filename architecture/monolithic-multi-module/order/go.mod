module github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/order

go 1.26

require (
	github.com/gin-gonic/gin v1.12.0
	github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment v0.0.0
	github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product v0.0.0
	github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared v0.0.0
	github.com/shopspring/decimal v1.6.0
	gorm.io/gorm v1.31.2
)

replace github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/shared => ../shared

replace github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/product => ../product

replace github.com/jamieleestudio/go-tutorials/architecture/monolithic-multi-module/payment => ../payment
