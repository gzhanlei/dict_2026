-- ============================================================================
-- 为 names / places 表添加预清洗查询列 name_for_query_clean 并建索引
--
-- 背景：应用查询原为
--   WHERE REPLACE(REPLACE(name_for_query_251202,'(',' '),')',' ') LIKE '% q %'
-- 对列套 REPLACE 导致任何索引都失效，且每行重复计算两次函数 → 全表扫描慢查询。
-- 预清洗列在存储层物化 REPLACE 结果，查询变为普通列 LIKE，不再逐行计算。
--
-- 配套：config/app.yaml 的 fields.query_field_clean。
--       列不存在时应用会自动回退旧 REPLACE 查询并打日志，先部署代码后执行本脚本也安全。
--
-- 前置检查：
--   1. MySQL 5.7+（生成列）；SELECT VERSION(); 确认
--   2. SHOW CREATE TABLE names_260202;  -- 确认原列类型/字符集/排序规则
--      生成列的类型必须显式声明且与原列一致（当前为 VARCHAR(100)，
--      字符集/排序规则继承表默认，与原列相同），保证 LIKE 匹配语义
--      （大小写、重音处理）与原 REPLACE 查询完全一致
--
-- 注意：表名带日期后缀，随数据更新变化，执行前替换为当前实际表名。
-- 本脚本已在 MySQL 9.6（names 67.7万行 / places 35.5万行）实测：
--   等价性 5 组查询（含括号场景）新旧计数全部一致；执行耗时 -31%（0.71s→0.48s）。
-- ============================================================================

-- ============ 方式一（推荐）：一次 ALTER，生成列 + 索引 ============
-- 生成列始终由原列推导，后续重建表/刷数据不会出现漏填。
-- ⚠️ STORED 生成列只支持 ALGORITHM=COPY：会重建表，期间阻塞写入（读不受影响）。
--    请在低峰期执行；若表很大且不能接受写入暂停，改用方式三。
-- 如果原列字符集是 utf8mb4 且长度超过 768 导致索引超长（3072 字节上限），
-- 改用前缀索引：ADD INDEX idx_name_for_query_clean (name_for_query_clean(255))

ALTER TABLE names_260202
  ADD COLUMN name_for_query_clean VARCHAR(100)
    GENERATED ALWAYS AS (REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ')) STORED,
  ADD INDEX idx_name_for_query_clean (name_for_query_clean);

ALTER TABLE places_260202
  ADD COLUMN name_for_query_clean VARCHAR(100)
    GENERATED ALWAYS AS (REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ')) STORED,
  ADD INDEX idx_name_for_query_clean (name_for_query_clean);

-- ============ 方式二：下次重建新表时，把列定义写进建表语句 ============
-- 在 CREATE TABLE 中加入（零额外开销，随建表一起完成）：
--   name_for_query_clean VARCHAR(100)  -- 类型与原列一致
--     GENERATED ALWAYS AS (REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ')) STORED,
--   INDEX idx_name_for_query_clean (name_for_query_clean)

-- ============ 方式三（大表在线，不推荐）：普通列 + 分批回填 + 在线索引 ============
-- 缺点：普通列不会随数据自动更新，若重建表/ETL 忘记写入会出现 NULL 漏匹配；
--       优先用方式一/二。
-- 步骤1（8.0 INSTANT 加列，几乎瞬时；类型/charset/collation 改为与原列完全一致）：
--   ALTER TABLE names_260202
--     ADD COLUMN name_for_query_clean VARCHAR(100)
--       CHARACTER SET utf8mb3 COLLATE utf8mb3_general_ci;
-- 步骤2（按主键分批回填，每批几万行，避免长事务）：
--   UPDATE names_260202
--     SET name_for_query_clean = REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ')
--     WHERE id > 0 AND id <= 100000;  -- 调整区间，重复直至回填完成
-- 步骤3（在线加索引，不阻塞 DML）：
--   ALTER TABLE names_260202 ADD INDEX idx_name_for_query_clean (name_for_query_clean);

-- ============================================================================
-- 执行后验证
-- ============================================================================

-- 1. 等价性验证：两条计数必须相等（多换几个查询词验证，含带括号的如 'Son of Zebedee'）
SELECT COUNT(*) FROM names_260202
WHERE REPLACE(REPLACE(name_for_query_251202, '(', ' '), ')', ' ') LIKE '% François Dupare %';

SELECT COUNT(*) FROM names_260202
WHERE name_for_query_clean LIKE '% François Dupare %';

-- 2. 确认列与索引已创建（应看到 name_for_query_clean 列和 idx_name_for_query_clean 索引）
SHOW CREATE TABLE names_260202;

-- 3. 执行计划对比：
--    旧查询每行计算 REPLACE×2；新查询 WHERE 子句是普通列 LIKE。
--    实测（MySQL 9.6，67.7万行）优化器仍选择表扫描（索引扫描+回表对它更贵），
--    收益来自去掉逐行函数计算（本地实测 -31%）；前导 % 的 LIKE 本质仍是扫描型。
EXPLAIN SELECT DISTINCT name, nation, translation FROM names_260202
WHERE name_for_query_clean LIKE '% François Dupare %' ORDER BY name LIMIT 100;
