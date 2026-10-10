# choice-matrix-backend

## 线上语言文件

管理员在项目工作台点击“线上语言文件 → 发布语言文件”时，服务端以上一版快照为底稿，只用当前 `published` 词条的非空译文覆盖对应的语言、模块和 key。缺失或仍在编辑的词条保留旧值。所有语言文件生成成功后，整个版本在一个数据库事务中生效；普通词条编辑和单条发布不会更新公开文件。

- `GET /api/v1/projects/:id/locale-release`：登录后读取当前版本；首次发布前返回 `data: null`。
- `POST /api/v1/projects/:id/locale-release/publish`：仅管理员可以手动发布。
- `GET /api/v1/public/locales/:projectCode/:languageCode.json`：无需登录的 JSON 文件链接。支持 ETag，稳定链接每次请求会校验是否有新版本。

首次部署需要创建 `locale_releases` 和 `locale_release_files` 两张表。启用 `AUTO_MIGRATE=true` 时由 GORM 创建；否则执行 `migrations/20260924_create_locale_releases.sql`。
