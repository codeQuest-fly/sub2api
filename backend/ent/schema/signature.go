// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Signature 定义 Signature 池实体的 schema。
//
// Signature 用于存储可用于流式响应中 thinking block 签名替换的有效签名。
// 这些签名从正常响应中收集或手动导入，用于替换/补充缺失的签名。
//
// 主要功能：
//   - 存储 Base64 编码的签名值
//   - 通过 SHA256 哈希实现去重
//   - 支持按模型、来源、状态分类管理
//   - 跟踪使用统计
type Signature struct {
	ent.Schema
}

// Annotations 返回 schema 的注解配置。
func (Signature) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "signatures"},
	}
}

// Mixin 返回该 schema 使用的混入组件。
func (Signature) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Fields 定义 Signature 实体的所有字段。
func (Signature) Fields() []ent.Field {
	return []ent.Field{
		// value: Base64 编码的签名值
		field.Text("value").
			NotEmpty().
			Comment("Base64 encoded signature value"),

		// hash: 签名值的 SHA256 哈希，用于去重
		field.String("hash").
			MaxLen(64).
			NotEmpty().
			Unique().
			Comment("SHA256 hash of signature value for deduplication"),

		// model: 关联的模型名称（可选，用于按模型分类）
		field.String("model").
			MaxLen(100).
			Optional().
			Nillable().
			Comment("Associated model name for categorization"),

		// source: 签名来源
		// - "collected": 从正常响应中自动收集
		// - "imported": 手动批量导入
		// - "manual": 手动单条添加
		field.Enum("source").
			Values("collected", "imported", "manual").
			Default("manual").
			Comment("Source of the signature"),

		// status: 签名状态
		// - "active": 可用
		// - "disabled": 已禁用
		// - "expired": 已过期（验证失败）
		field.Enum("status").
			Values("active", "disabled", "expired").
			Default("active").
			Comment("Signature status"),

		// use_count: 使用次数统计
		field.Int64("use_count").
			Default(0).
			Comment("Number of times this signature has been used"),

		// last_used_at: 最后使用时间
		field.Time("last_used_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("Last time this signature was used"),

		// last_verified_at: 最后验证时间
		field.Time("last_verified_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
			Comment("Last time this signature was verified as valid"),

		// notes: 备注信息
		field.String("notes").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Comment("Additional notes"),

		// collected_from_account_id: 采集来源账号ID
		field.Int64("collected_from_account_id").
			Optional().
			Nillable().
			Comment("Account ID from which this signature was collected"),
	}
}

// Indexes 定义数据库索引，优化查询性能。
func (Signature) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),                     // 按状态筛选
		index.Fields("source"),                     // 按来源筛选
		index.Fields("model"),                      // 按模型筛选
		index.Fields("use_count"),                  // 按使用次数排序
		index.Fields("last_used_at"),               // 按最后使用时间排序
		index.Fields("deleted_at"),                 // 软删除查询优化
		index.Fields("collected_from_account_id"),  // 按采集来源账号筛选
	}
}
