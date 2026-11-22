#!/bin/bash
# queries 表定期清理：分批删除超过保留期的查询日志行
#
# 背景：/api/search 每个请求都会异步写入一条 queries（含缓存命中），表只增不减，
# 需定期清理。分批 DELETE 是为了控制锁持有时间与单事务 binlog 体积；
# 依赖 query_time 上的索引（无索引则每批都会全表扫描，见 AGENTS.md）。
# 长期趋势统计由 rollup_queries_daily.sh 写入 queries_daily，不受本清理影响。
#
# 部署：服务器 crontab（root）
#   0 4 * * * /data/www/dict20260412/scripts/cleanup_queries.sh
# 凭据：/root/.dict_maint.cnf（0600，[client] host/port/user/password），
#       不把密码写进脚本或 crontab。
#
# 手动首跑（历史积压多时可调大批次、去掉间隔）：
#   BATCH=20000 BATCH_SLEEP=0.2 /data/www/dict20260412/scripts/cleanup_queries.sh
set -euo pipefail

CNF="${CNF:-/root/.dict_maint.cnf}"          # mysql --defaults-extra-file 凭据文件
DB="${DB:-humanname}"                        # 生产库名
RETENTION_DAYS="${RETENTION_DAYS:-180}"      # 明细保留天数（更早的靠 queries_daily 留趋势）
BATCH="${BATCH:-5000}"                       # 单批删除行数
BATCH_SLEEP="${BATCH_SLEEP:-1}"              # 批间隔秒数，给数据库留喘息
LOG="${LOG:-/var/log/dict_queries_cleanup.log}"

log() { echo "$(date '+%F %T') $*" >> "$LOG"; }
trap 'log "cleanup FAILED (retention=${RETENTION_DAYS}d, deleted=${total} before error)"' ERR

total=0
while :; do
    n=$(mysql --defaults-extra-file="$CNF" "$DB" -N -B -e \
        "DELETE FROM queries WHERE query_time < NOW() - INTERVAL ${RETENTION_DAYS} DAY LIMIT ${BATCH};
         SELECT ROW_COUNT();")
    total=$((total + n))
    # 一批删不满说明已删完，结束（幂等：重复运行第二次即删 0 行退出）
    [ "$n" -lt "$BATCH" ] && break
    sleep "$BATCH_SLEEP"
done

log "cleanup ok retention=${RETENTION_DAYS}d deleted=${total}"
