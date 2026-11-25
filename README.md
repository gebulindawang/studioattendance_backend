# StudioAttendance 后端

Go 版本见 `go.mod`。数据库使用 MySQL 8.0。开发前端的 Vite 代理默认将 `/api` 转发到 `127.0.0.1:8080`。

## 初始化与升级

1. 创建目标数据库并选中它，然后通过 MySQL 客户端执行 [docs/sql/init.sql](docs/sql/init.sql)。脚本可重复执行，保留已有数据。已部署版本升级时也需执行，以补齐课表图片和工作室签到字段。旧版的 `user.sql` 和 `schedules.sql` 是历史导出。
2. 在后端运行环境中配置 `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD` 和 `DB_NAME`。将 `JWT_SECRET` 设置为至少 32 字节的新随机密钥。升级后需要重新登录。考勤默认使用 `Asia/Shanghai`，可通过 `ATTENDANCE_TIME_ZONE` 配置。
3. 在后端目录运行 `go run ./cmd/server`。图片上传和识别分别使用 `OSS_*` 和 `DEEPSEEK_*` 配置。
4. 新库中先通过注册页创建账号。数据库管理员核实该账号后，可用下面的 SQL 将其提升为管理员。首次登录后台后，在「学期配置」填写日期并添加实际使用的大节。

```sql
UPDATE users
SET role = 1
WHERE username = '<已核实的管理员用户名>' AND deleted_at IS NULL;
```

旧版公开注册曾将新账号设为管理员。开放升级后的系统前，核对 `users` 中所有 `role = 1` 的账号，将确认属于普通成员的账号改为 `role = 0`。账号启停和角色以数据库当前值为准。没有历史时间戳的课表图片会显示为时间未知，已确认的课程仍可使用。

如果旧库缺少 `schedules_image.created_at` 或 `reviewed`，管理员仍可查看已有课表和图片。请在服务所连接的数据库执行 [docs/sql/init.sql](docs/sql/init.sql)，补齐上传和审核所需的字段。旧图片已有的 `create_time` 可继续用于显示上传时间。

### 恢复旧账号的启用状态

旧版注册可能将 `status` 写为 `0`，当时登录没有检查这个字段。新版登录要求 `status = 1`，请在升级前核对旧账号的启用状态。历史行里的 `0` 也可能表示有意禁用。

```sql
SELECT id, username, role, status
FROM users
WHERE deleted_at IS NULL
ORDER BY role DESC, id;
```

如果所有管理员都无法登录，请先核实管理员身份和角色，再由数据库管理员恢复一个确认应启用的管理员。用实际用户名替换下面的占位符。

```sql
UPDATE users
SET status = 1
WHERE username = '<已核实且应启用的管理员用户名>'
  AND role = 1 AND status = 0 AND deleted_at IS NULL;
```

恢复登录后，在「成员管理」中启用已核实的普通成员。

## 考勤规则

管理员确认课表时，后端会在同一个事务里生成该成员本周的签到表。在「考勤记录」也可以选择成员，通过 `PUT /api/record/record` 传入 `{"user_ids":[1,2]}` 生成新一周的签到表，或补生成已确认成员的记录。接口批量处理服务器所在考勤时区的自然周（周一至周日），并返回新增、调整、移除数量和各成员处理结果。学期配置中有效的大节作为候选时段。未上传和待解析的成员有各自的提示。

一个大节对应两个小节。任一小节有课时，整个大节跳过。课表修改后可重新生成，服务器会更新尚未开放的待签到格子，保留已签到和已开放的记录。

成员通过 `GET /api/record/week` 读取本周签到表，通过 `POST /api/record/:id/checkin` 签到。每格可以在大节开始前 5 分钟内签到一次，开始时截止。签到只更新登录成员已有的待签到记录。未签到且已截止的格子显示为「签到已截止」，旧版考勤记录仍可在管理端查询。
