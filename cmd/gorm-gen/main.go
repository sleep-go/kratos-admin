package main

import (
	"log"

	"dps/internal/data/model"

	"gorm.io/gen"
)

// main 生成 GORM Gen 类型安全查询代码。
// 运行：go run ./cmd/gorm-gen
func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath:      "internal/data/query",
		ModelPkgPath: "dps/internal/data/model",
		Mode:         gen.WithDefaultQuery | gen.WithQueryInterface,
	})

	g.ApplyBasic(model.Admin{})
	g.Execute()

	log.Println("gen: query code generated to internal/data/query")
}
