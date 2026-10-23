# Correctness Framework 开发指南

本框架维护三项相互独立的证据：

- [统计语义契约](counting-semantics.md)：locq 应该如何分类每个物理行。
- [语言矩阵](language-test-matrix.md)：各语言承诺测试哪些能力，哪些保持历史行为。
- `scripts/diffcheck`：哪些文件与外部工具不同，有哪些可核对的事实及待调查项。

它不修改正常 CLI 的执行路径，也不以对齐 tokei/cloc 的总数作为验收标准。

## 本地正确性检查

```sh
go test -race -tags=integration ./...
go test ./internal/core -run '^$' -fuzz '^FuzzLanguageReader$' -fuzztime=30s
go test ./internal/lexer -run '^$' -fuzz '^FuzzScannerCatalog$' -fuzztime=30s
```

逐行 golden 复用 `lexertest.Case`。全部语言进行读取方式/调试路径/状态隔离检查；
所有已启用规则测试声明的注释、字符串和适用的嵌套/文档结构。特性模板的期望
由统计契约写死，不取自扫描结果。`matrix.go` 额外保存语义规则指纹，防止规则
修改顺带改写模板输入并自动通过。指纹是评审入口，不是正确性 oracle；优化索引
和 fast-path 开关不参与该指纹。

新增语言或改规则时：先人工推导逐行 Kinds，补正反用例和边界，再评审适用性及
规则指纹。`go generate ./...` 不会更新 Kinds 或矩阵基线。生成物的新鲜度、快照
哈希与完整名称映射已经由现有生成测试检查，无需另一套 CI 生成器。

```sh
go run ./scripts/diffcheck -matrix docs/language-test-matrix.md
```

## 固定语料上的跨工具对比

报告目录必须是全新目录，且位于扫描根目录以外。已有目录不会被覆盖。
默认比较 locq 与 tokei，`-cloc cloc` 可加入 cloc；locq 使用当前构建的库实现。

```sh
go run ./scripts/diffcheck -export-corpus /tmp/locq-contract-corpus
go run ./scripts/diffcheck \
  -root /tmp/locq-contract-corpus/src \
  -golden /tmp/locq-contract-corpus/golden.json \
  -policy aligned -cloc cloc -out /tmp/locq-contract-report
```

`golden.json` 放在 `src/` 外，避免测试期望本身混入扫描输入。基准固定语料的内容
指纹不变；正确性 corpus 独立扩展。Starlark 样本使用 locq 已支持的 `.star` 后缀；
词法支持不等于支持外部工具的所有扩展名。

默认保留各工具的忽略/去重行为；`-policy aligned` 显式为 tokei 设置
`--no-ignore --hidden`，为 cloc 设置 `--skip-uniqueness --timeout 0`。
剩余的检测范围、VCS 目录和二进制判定差异仍然如实报告，不声称这些参数能统一
所有文件集合。每个外部工具还有 `-timeout` 指定的进程截止时间，默认 2 分钟。

报告包含：原始 JSON、stderr、实际版本/参数、输入文件 SHA-256 清单、整体输入
指纹、规则版本、JSON/Markdown 差异、原文件复现副本及 locq 的逐行轨迹。
若扫描根就是 Git 仓库根，还记录 commit 和 dirty 状态；检测到输入变化则报告
不完整。固定仓库可用 `git archive <SHA>` 导出，另行保存其来源/commit 元数据。
为保证输入指纹绑定实际读取的字节，差异工具拒绝符号链接及其他非普通文件；请使用
完整导出的源码快照。此限制不改变 locq 正常 CLI 的文件处理行为。

默认只报告差异。`-check` 在未知/不允许的差异、失效已知记录或不完整执行时失败。
报告产生失败、输入错误等执行问题始终返回非零。

## 归因顺序与证据边界

先比较文件集合，再对共同文件比较规范化语言，最后比较同语言的计数：

| 类别 | 证据 |
| --- | --- |
| FILE_SET | 完整输出中，仅一方包含该路径；不能据此断言“被忽略” |
| LANGUAGE_MAPPING | 同一路径的规范化语言不同 |
| LEXICAL_COUNT | 同一路径、同语言的 Code/Comment/Blank 不同 |
| EMBEDDED_LANGUAGE | 外部 JSON 实际报告了嵌入语言 blob；宿主与嵌入计数分别保留 |
| IO_OR_TIMEOUT | 工具失败、超时、读取诊断或输入变化；不据此推断文件缺失原因 |
| KNOWN_SEMANTIC_DIFF | 精确命中已核对的版本、源码、分类和两方计数记录 |
| UNCLASSIFIED | JSON 无法解释或路径不属于输入清单等，保留待调查 |

不完整执行仍保存原始输出；不会将不完整输出中的“缺文件”批量当作 FILE_SET。

**tokei/cloc 的逐文件 JSON 没有逐行分类。** 报告中的 Reference 列用 `?` 表示未知，
不能通过总数反推“第几行一定不同”。locq 的行号、原文、分类以及可用的独立 golden
均可直接核对。完整文件副本保持跨行上下文；报告仅预览前 32 行，完整轨迹见
相邻 `.lines.json`。独立 golden 作为额外 `contract` 参考，可以定位确切契约违例。
每个 golden 都比较逐行标签，即使 Code/Comment/Blank 总数相同也不会漏掉标签互换。

## 精确记录已知差异

`scripts/diffcheck/known-differences.json` 每条锁定：唯一 ID、精确路径、原始差异
类别、解释、Issue 或本地决策文档引用、参考工具及实际版本、locq 规则版本、源码
SHA-256，以及两方完整规范化文件记录。`allowed` 控制严格模式是否接受该差异，
不会从报告删除记录。没有通配文件忽略，也不会只匹配某个 Code 差值。

任何版本、源码或计数改变都使旧记录失效；未命中记录明确显示在报告中，严格
模式会失败。更新记录前必须重新核对统计语义，不能把未知差异批量写成 allowed。

### 已核对：Plain Text

locq 将非空文本计为 Code，tokei 将其计为 Comment。固定的三行文本 golden
`some text`、空行、`// still text` 分别为 `CBC`，所以该差异可以精确记录。
这一结论不授权忽略其他文本文件、其他版本或其他计数表现。

## 重放结构化输出

运行产生的 `report.json` 绑定实际原始 JSON 的 SHA-256、工具版本和执行完整性。
导入时必须同时提供它；裸 JSON 无法证明没有超时或错误：

```sh
go run ./scripts/diffcheck \
  -root /tmp/locq-contract-corpus/src \
  -locq-json /tmp/locq-contract-report/locq.json \
  -tokei-json /tmp/locq-contract-report/tokei.json \
  -metadata /tmp/locq-contract-report/report.json \
  -out /tmp/locq-contract-replay
```

输入内容或原始 JSON 被修改时，重放会拒绝伪装成原有完整执行。外部格式更改导致
无法规范化时保留 UNCLASSIFIED，不静默使用零值。规范化拒绝重复路径、越界路径、
缺少计数字段、负数或小数计数。
不稳定输入的报告、未知 schema 或语义版本不能重放。相同内容可迁移目录，路径按
原报告根目录规范化；只有当前可执行文件指纹匹配时，才为导入结果附加逐行轨迹。
每个工具还保留原始 JSON 的路径根目录，因此迁移后的报告可以继续重放。

## CI 分工

普通 PR 和语言规则更新沿用三平台 `go test -race -tags=integration ./...`，
自动覆盖新契约、语言矩阵、生成校验和属性回归，不安装外部统计工具。

周期/手动 correctness 工作流固定 tokei 15.0.0、cloc 2.04、Go 仓库 SHA，分别
扫描独立 corpus 与固定仓库，上传可重放报告。工具运行不完整或输入变化会使
周期任务失败，统计差异仍只报告。扩展 fuzz 是独立 job，也由发布流程调用；
跨平台发布检查继续复用现有工作流。性能仍由现有 Benchmarks 工作流独立测量，
避免与正确性或模糊测试争用资源。

当前固定 Go 快照上，cloc 2.04 默认单文件超时会跳过 `src/net/http/requestwrite_test.go`
及 `src/vendor/golang.org/x/net/idna/tables15.0.0.go`、`tables17.0.0.go`。关闭该超时后，
本机整次扫描触发 5 分钟截止时间。因此完整 Go/cloc 对比尚无完整结果，报告保留
IO_OR_TIMEOUT；受控 corpus 的 cloc 对比可完整执行。这不是允许忽略这些文件的规则。

## 缩小词法差异复现

```sh
go run ./scripts/diffcheck -root /path/to/source \
  -minimize relative/file.go -tokei tokei \
  -out /tmp/locq-minimized -timeout 2m -minimize-budget 100
```

此模式先在隔离目录验证原文件能单独复现，再按连续物理行片段删除，保留相同
语言及完全相同的 Code/Comment/Blank 差值。只处理两边完整、同语言且没有嵌入
blob 的词法差异；文件集合、语言检测或依赖仓库上下文的差异不会被伪装成词法
复现。`-cloc cloc` 可选择 cloc 作为参考。

输出保存原始字节、缩小后的源码、保留下来的原行号、工具版本和计数证据。
`one_line_deletion_minimal` 仅表示已验证再删除任意一行都会失去当前差异；预算或
截止时间耗尽时不会宣称最小。它不证明语法最小或差异原因，需要开发者按统计
契约确认并补充独立 golden。
