#!/bin/bash
# queries 每日汇总：把"昨天"的查询日志聚合成一天一行，写入 queries_daily
#
# 目的：queries 明细因 cleanup_queries.sh 只保留有限天数，长期统计趋势
#（每日请求量 / 独立IP数 / 独立查询词数，按 queryType 分列）靠本表保留，
# 一年仅 ~365*3 行。REPLACE 保证幂等，重跑/补跑同一天不会产生重复行。
# 桶边界按数据库会话时区的 DATE(query_time) 划分，日与日之间口径一致。
#
# 部署：服务器 crontab（root）
#   5 0 * * * /data/www/dict20260412/scripts/rollup_queries_daily.sh
# 凭据：同 cleanup_queries.sh（/root/.dict_maint.cnf）
# 历史回填：本脚本只管"昨天"；存量历史的回填见仓库 scripts/ 下部署记录
set -euo pipefail

CNF="${CNF:-/root/.dict_maint.cnf}"
DB="${DB:-humanname}"
LOG="${LOG:-/var/log/dict_queries_rollup.log}"

log() { echo "$(date '+%F %T') $*" >> "$LOG"; }
trap 'log "rollup FAILED"' ERR

mysql --defaults-extra-file="$CNF" "$DB" <<'SQL'
CREATE TABLE IF NOT EXISTS queries_daily (
    day            DATE        NOT NULL,
    query_type     VARCHAR(10) NOT NULL,  -- person / place / both
    cnt            BIGINT      NOT NULL,  -- 当日请求条数
    distinct_ips   INT         NOT NULL,  -- 当日独立IP数
    distinct_words INT         NOT NULL,  -- 当日独立查询词数
    PRIMARY KEY (day, query_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

REPLACE INTO queries_daily (day, query_type, cnt, distinct_ips, distinct_words)
SELECT DATE(query_time), queryType, COUNT(*), COUNT(DISTINCT ip), COUNT(DISTINCT query_word)
FROM queries
WHERE query_time >= CURDATE() - INTERVAL 1 DAY
  AND query_time <  CURDATE()
GROUP BY 1, 2;
SQL

rows=$(mysql --defaults-extra-file="$CNF" "$DB" -N -B -e \
    "SELECT IFNULL(SUM(cnt),0) FROM queries_daily WHERE day = CURDATE() - INTERVAL 1 DAY;")
log "rollup ok day=$(date -v-1d '+%F' 2>/dev/null || date -d yesterday '+%F') requests=${rows}"
