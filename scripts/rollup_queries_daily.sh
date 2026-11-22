#!/bin/bash
# queries 每日汇总：把"昨天"的查询日志聚合成两张统计表，写入"昨天"一行
#
# 目的：queries 明细因 cleanup_queries.sh 只保留有限天数，长期统计靠汇总表：
# - queries_daily       每日请求量 / 独立IP数 / 独立查询词数（按 queryType 分列）
# - queries_words_daily 每日每个查询词的次数与独立IP数（支持"最热查询词"统计，
#   含被清理掉的明细的历史）
# 两张表均 REPLACE 幂等：重跑/补跑同一天不会产生重复或翻倍。一年约几万行。
# 桶边界按数据库会话时区的 DATE(query_time) 划分，日与日之间口径一致。
#
# 部署：服务器 crontab（root）
#   5 0 * * * /data/www/dict20260412/scripts/rollup_queries_daily.sh
# 凭据：同 cleanup_queries.sh（/root/.dict_maint.cnf）
# 历史回填：本脚本只管"昨天"；存量历史（含已清理行）已于 2026-11-22 从
# 备份 dump + 当时存量一次性回填，见仓库提交 129e93e 之后的部署记录
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

CREATE TABLE IF NOT EXISTS queries_words_daily (
    day          DATE         NOT NULL,
    query_type   VARCHAR(10)  NOT NULL,  -- person / place / both
    query_word   VARCHAR(200) NOT NULL,  -- TRIM 后的查询词（大小写经排序规则归并）
    cnt          INT          NOT NULL,  -- 当日该词请求次数
    distinct_ips INT          NOT NULL,  -- 当日查过该词的独立IP数
    PRIMARY KEY (day, query_type, query_word)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
-- collation 必须用 *_general_ci（区分重音，与源表 utf8mb3_general_ci 语义一致）：
-- 默认的 *_0900_ai_ci 会把 jose/josé 这类重音变体折成同一个 PK 键，REPLACE 时
-- 静默丢计数（2026-11-22 回填时实测丢了 202 条后改为 general_ci）

REPLACE INTO queries_daily (day, query_type, cnt, distinct_ips, distinct_words)
SELECT DATE(query_time), queryType, COUNT(*), COUNT(DISTINCT ip), COUNT(DISTINCT query_word)
FROM queries
WHERE query_time >= CURDATE() - INTERVAL 1 DAY
  AND query_time <  CURDATE()
GROUP BY 1, 2;

REPLACE INTO queries_words_daily (day, query_type, query_word, cnt, distinct_ips)
SELECT DATE(query_time), queryType, TRIM(query_word), COUNT(*), COUNT(DISTINCT ip)
FROM queries
WHERE query_time >= CURDATE() - INTERVAL 1 DAY
  AND query_time <  CURDATE()
GROUP BY 1, 2, 3;
SQL

rows=$(mysql --defaults-extra-file="$CNF" "$DB" -N -B -e \
    "SELECT IFNULL(SUM(cnt),0) FROM queries_daily WHERE day = CURDATE() - INTERVAL 1 DAY;")
words=$(mysql --defaults-extra-file="$CNF" "$DB" -N -B -e \
    "SELECT COUNT(*) FROM queries_words_daily WHERE day = CURDATE() - INTERVAL 1 DAY;")
log "rollup ok day=$(date -v-1d '+%F' 2>/dev/null || date -d yesterday '+%F') requests=${rows} word_rows=${words}"
