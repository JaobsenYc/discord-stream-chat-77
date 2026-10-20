# sql/migrations

**全新部署不需要本目录。** 完整 schema(001-006)在 [`sql/init/`](../init/),由
Postgres 容器首次启动时自动执行,基线会登记到 `schema_migrations` 表。

本目录只存放**上线后**的增量变更,从 `007_` 开始编号:

```bash
make db-create-migration NAME="add_xxx"   # 生成 007_add_xxx.sql
make db-migrate                           # 应用未执行的迁移
make db-status                            # 查看迁移状态
```

规则:迁移只增不改——历史文件(含 sql/init 内的基线)一旦执行过就不要再编辑,
新变更永远写新文件。
