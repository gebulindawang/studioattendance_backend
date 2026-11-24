# StudioAttendance 后端

Go 版本见 `go.mod`。数据库使用 MySQL 8.0。开发前端的 Vite 代理默认将 `/api` 转发到 `127.0.0.1:8080`。

## 初始化与升级

1. 创建目标数据库并选中它，然后通过 MySQL 客户端执行 [docs/sql/init.sql](docs/sql/init.sql)。脚本可重复执行，保留已有数据。已部署版本也需执行一次以补齐课表图片字段。旧版的 `user.sql` 和 `schedules.sql` 是历史导出。
2. 在后端运行环境中配置 `DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD` 和 `DB_NAME`。将 `JWT_SECRET` 设置为至少 32 字节的新随机密钥。升级后需要重新登录。考勤默认使用 `Asia/Shanghai`，可通过 `ATTENDANCE_TIME_ZONE` 配置。
3. 在后端目录运行 `go run ./cmd/server`。图片上传和识别分别使用 `OSS_*` 和 `DEEPSEEK_*` 配置。
4. 新库中先通过注册页创建账号。数据库管理员核实该账号后，可用下面的 SQL 将其提升为管理员。首次登录后台后，在「学期配置」填写日期并添加实际使用的大节。

```sql
UPDATE users
SET role = 1
WHERE username = '<已核实的管理员用户名>' AND deleted_at IS NULL;
```

旧版公开注册曾将新账号设为管理员。开放升级后的系统前，核对 `users` 中所有 `role = 1` 的账号，将确认属于普通成员的账号改为 `role = 0`。账号启停和角色以数据库当前值为准。没有历史时间戳的课表图片会显示为时间未知，已确认的课程仍可使用。

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

每个配置的大节对应两个小节。已确认课表的在职成员在学期内按小节考勤，有课时豁免。签到从小节开始前 20 分钟开放，开课后 5 分钟截止。服务每分钟结算当天已结束的节次，补写缺勤或有课豁免，已有记录会保留。服务停机时错过的历史日期需要按当时的配置和课表核对。

成员通过 `GET /api/record/today` 查看当日安排，通过 `POST /api/record/checkin/:section` 签到。登录身份由服务端判断。
