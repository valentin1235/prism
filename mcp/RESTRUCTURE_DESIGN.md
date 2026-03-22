# Prism MCP 서버 패키지 재구조화 설계

> 설계 일자: 2026-03-22
> 대상: ~/prism/mcp (현재 flat `package main`, 19개 소스 파일, 7,868 LOC)
> 참고: ~/ouroboros/src/ouroboros/mcp (Python, client/server/tools/resources 분리)

---

## 1. 목표 디렉토리 트리

```
mcp/
├── main.go                              # 엔트리포인트 (slim: init + server setup + tool registration)
├── go.mod                               # module github.com/heechul/prism-mcp
├── go.sum
│
├── internal/
│   ├── engine/                          # Claude CLI 상호작용 계층
│   │   ├── claude.go                    #   ClaudeOptions, Query, QuerySync, ResultMessage
│   │   └── llm.go                       #   callLLM 유틸리티, filterEnv, model validation
│   │
│   ├── task/                            # 태스크 수명주기 관리
│   │   └── store.go                     #   TaskStore, AnalysisTask, TaskStatus, StageStatus,
│   │                                    #   StageName, TaskSnapshot, StageProgress
│   │
│   ├── parallel/                        # 병렬 실행 인프라
│   │   └── executor.go                  #   ParallelExecutor, ParallelJob, ParallelResults
│   │
│   ├── pipeline/                        # 4단계 분석 파이프라인
│   │   ├── config.go                    #   AnalysisConfig, readAnalysisConfig, StageResult
│   │   ├── orchestrator.go              #   runAnalysisPipeline (4단계 순차 실행 루프)
│   │   │
│   │   ├── scope.go                     #   Stage 1: runSeedAnalysis, BuildSeedAnalystPrompt,
│   │   │                                #            BuildPerspectiveGeneratorPrompt,
│   │   │                                #            LoadOntologyDocPaths, schema 상수들
│   │   ├── specialist.go                #   Stage 2: BuildAllSpecialistCommands,
│   │   │                                #            BuildSpecialistCommand, SpecialistCommand,
│   │   │                                #            LoadSpecialistContext, SpecialistContext
│   │   ├── interview.go                 #   Stage 3: BuildAllInterviewCommands,
│   │   │                                #            BuildInterviewCommand, InterviewCommand,
│   │   │                                #            LoadInterviewContext, InterviewContext
│   │   ├── synthesis.go                 #   Stage 4: SynthesisContext, LoadSynthesisContext,
│   │   │                                #            runSynthesis, buildSynthesisPrompt
│   │   │
│   │   ├── seed.go                      #   SeedAnalysis, SeedFinding, SeedResearch, SeedPatch,
│   │   │                                #   MergeSeedAnalysis, PatchSeedAnalysisFile,
│   │   │                                #   ReadSeedAnalysis, WriteSeedAnalysis
│   │   ├── perspectives.go              #   Perspective, AnalystPrompt, PerspectivesOutput,
│   │   │                                #   ReadPerspectives, WritePerspectives,
│   │   │                                #   ValidatePerspectives, PerspectiveQualityGate
│   │   └── collector.go                 #   SpecialistResult, SpecialistOutcome,
│   │                                    #   SpecialistFindings, SpecialistFinding,
│   │                                    #   CollectedFindings, CollectedVerifications,
│   │                                    #   CollectSpecialistResults, CollectInterviewResults,
│   │                                    #   InterviewResult, VerifiedFindings 등 I/O 함수
│   │
│   └── handler/                         # MCP 도구 핸들러 (thin adapter 계층)
│       ├── analyze.go                   #   handleAnalyze, handleTaskStatus,
│       │                                #   handleAnalyzeResult, handleCancelTask,
│       │                                #   extractReportSummary, extractSection
│       ├── interview.go                 #   handleInterview, handleScore,
│       │                                #   InterviewSession, QA, InterviewResponse, scoreSession
│       ├── review.go                    #   handleDAReview, DAFinding, DAReviewResult
│       └── docs.go                      #   initFilesystem, handleListRoots, handleListDir,
│                                        #   handleReadFile, handleSearchFiles, allowedDirs
```

### 테스트 파일 배치 (관례: 동일 디렉토리)

```
internal/
├── engine/
│   └── (현재 engine 관련 테스트 없음 — 추후 추가)
├── task/
│   └── store_test.go                    # ← task_manager_test.go
├── parallel/
│   ├── executor_test.go                 # ← parallel_test.go
│   └── safety_test.go                   # ← parallel_safety_test.go
├── pipeline/
│   ├── scope_test.go                    # ← stage1_test.go + stage1_exec_test.go
│   ├── specialist_test.go              # ← stage2_test.go + stage2_exec_test.go
│   ├── interview_test.go               # ← stage3_test.go
│   ├── synthesis_test.go               # ← stage4_exec_test.go
│   ├── seed_test.go                     # ← seed_merge_test.go
│   ├── perspectives_test.go            # ← perspectives_test.go
│   ├── collector_test.go               # ← result_collector_test.go
│   └── orchestrator_test.go            # ← pipeline_test.go
└── handler/
    ├── analyze_test.go                  # ← analyze_test.go + analyze_result_test.go
    └── review_test.go                   # ← da_review_test.go
```

---

## 2. 파일 이동 매핑 테이블

| # | 현재 파일 | LOC | 이동 목적지 | 새 패키지명 | 비고 |
|---|----------|-----|-----------|------------|------|
| 1 | `main.go` | 137 | `main.go` (그대로) | `main` | tool registration 유지, import 경로만 변경 |
| 2 | `claude_sdk.go` | 308 | `internal/engine/claude.go` | `engine` | ClaudeOptions, Query, QuerySync |
| 3 | `llm.go` | 173 | `internal/engine/llm.go` | `engine` | filterEnv, model validation 유틸리티 |
| 4 | `task_manager.go` | 392 | `internal/task/store.go` | `task` | TaskStore, AnalysisTask, 모든 Status 타입 |
| 5 | `parallel.go` | 197 | `internal/parallel/executor.go` | `parallel` | ParallelExecutor, ParallelJob |
| 6 | `analyze.go` | 937 | **분할** | — | 아래 3개 파일로 분할 |
| 6a | — (handle* 함수들) | ~350 | `internal/handler/analyze.go` | `handler` | handleAnalyze/TaskStatus/Result/Cancel |
| 6b | — (AnalysisConfig, StageResult) | ~50 | `internal/pipeline/config.go` | `pipeline` | 공유 설정 구조체 |
| 6c | — (runAnalysisPipeline) | ~450 | `internal/pipeline/orchestrator.go` | `pipeline` | 4단계 오케스트레이션 루프 |
| 7 | `stage1.go` | 435 | `internal/pipeline/scope.go` | `pipeline` | BuildSeedAnalystPrompt, BuildPerspectiveGeneratorPrompt |
| 8 | `stage1_exec.go` | 338 | `internal/pipeline/scope.go` (병합) | `pipeline` | runSeedAnalysis — stage1.go와 합침 |
| 9 | `stage2.go` | 506 | `internal/pipeline/specialist.go` | `pipeline` | BuildSpecialistCommand, SpecialistContext |
| 10 | `stage2_exec.go` | 70 | `internal/pipeline/specialist.go` (병합) | `pipeline` | BuildAllSpecialistCommands — stage2.go와 합침 |
| 11 | `stage3.go` | 484 | `internal/pipeline/interview.go` | `pipeline` | BuildAllInterviewCommands, InterviewCommand |
| 12 | `stage4_exec.go` | 463 | `internal/pipeline/synthesis.go` | `pipeline` | SynthesisContext, runSynthesis |
| 13 | `seed_merge.go` | 161 | `internal/pipeline/seed.go` | `pipeline` | SeedAnalysis 타입 + merge 로직 |
| 14 | `perspectives.go` | 233 | `internal/pipeline/perspectives.go` | `pipeline` | Perspective 타입 + I/O + validation |
| 15 | `result_collector.go` | 828 | `internal/pipeline/collector.go` | `pipeline` | SpecialistResult, CollectedFindings 등 |
| 16 | `interview.go` | 332 | `internal/handler/interview.go` | `handler` | handleInterview (MCP 도구 핸들러) |
| 17 | `scorer.go` | 76 | `internal/handler/interview.go` (병합) | `handler` | scoreSession — interview와 합침 |
| 18 | `da_review.go` | 394 | `internal/handler/review.go` | `handler` | handleDAReview, DAFinding |
| 19 | `filesystem.go` | 204 | `internal/handler/docs.go` | `handler` | initFilesystem, prism_docs_* 핸들러 |

**소스 파일 수 변화**: 19개 flat → 14개 파일 across 5 패키지 (병합으로 파일 수 감소)

---

## 3. 패키지 간 의존 관계 (Import 방향)

```
                    ┌──────────┐
                    │   main   │
                    └────┬─────┘
                         │ imports
              ┌──────────┼──────────┐
              ▼          ▼          ▼
        ┌─────────┐ ┌────────┐ ┌──────┐
        │ handler │ │  task  │ │engine│
        └────┬────┘ └────────┘ └──────┘
             │ imports    ▲         ▲
             ▼            │         │
        ┌──────────┐      │         │
        │ pipeline │──────┘─────────┘
        └────┬─────┘
             │ imports
             ▼
        ┌──────────┐
        │ parallel │
        └──────────┘
```

### Import 방향 상세

| From (importer) | To (imported) | 사용하는 주요 심볼 |
|-----------------|---------------|-------------------|
| `main` | `handler` | handleAnalyze, handleInterview, handleDAReview, handleScore, initFilesystem, handle* docs |
| `main` | `task` | NewTaskStore() — taskStore 초기화 |
| `main` | `engine` | (간접: handler를 통해) |
| `handler` | `pipeline` | RunAnalysisPipeline, AnalysisConfig |
| `handler` | `task` | TaskStore.Get/Create/Remove, TaskSnapshot |
| `handler` | `engine` | callLLM (da_review, interview, scorer에서 사용) |
| `pipeline` | `engine` | QuerySync, ClaudeOptions (각 stage에서 Claude CLI 호출) |
| `pipeline` | `task` | AnalysisTask.StartStage/CompleteStage/FailStage 등 상태 업데이트 |
| `pipeline` | `parallel` | ParallelExecutor.Execute (stage2, stage3에서 병렬 실행) |
| `parallel` | — | 외부 의존 없음 (context, sync만 사용) |
| `task` | — | 외부 의존 없음 (crypto/rand, sync만 사용) |
| `engine` | — | 외부 의존 없음 (os/exec, encoding/json만 사용) |

### 순환 의존 검증

```
main → handler → pipeline → parallel    ✅ 단방향
main → handler → task                    ✅ 단방향
main → handler → engine                  ✅ 단방향
pipeline → task                          ✅ 단방향
pipeline → engine                        ✅ 단방향
pipeline → parallel                      ✅ 단방향
```

**순환 없음**: 의존 그래프는 DAG(Directed Acyclic Graph)를 형성한다.
리프 패키지(`engine`, `task`, `parallel`)는 서로 의존하지 않는다.

---

## 4. 관심사 분리 근거

| 패키지 | 관심사 | 응집도 기준 |
|--------|--------|------------|
| **engine** | Claude CLI 프로세스 실행, NDJSON 파싱, 환경변수 관리 | LLM 상호작용 추상화 |
| **task** | 태스크 생성/조회/삭제, 상태 머신(queued→running→completed/failed), poll 카운터, 스냅샷 | 태스크 수명주기 |
| **parallel** | 세마포어 기반 동시성 제한, per-job 타임아웃, 자동 재시도 | 동시 실행 인프라 |
| **pipeline** | 4단계 파이프라인 오케스트레이션, 각 단계별 프롬프트 구성·실행·결과 수집, 도메인 타입(Perspective, SeedAnalysis, SpecialistFindings, VerifiedFindings, CollectedFindings) | 분석 도메인 로직 |
| **handler** | MCP 프로토콜 어댑터: request 파싱 → 도메인 로직 호출 → response 구성 | 외부 인터페이스 (MCP 도구) |

### ouroboros 구조와의 대응

| ouroboros (Python) | prism (Go) | 설계 판단 |
|-------------------|----------------|----------|
| `tools/` (핸들러 모음) | `handler/` | 동일 관심사. Go에서는 `tools`가 표준 라이브러리와 혼동 가능하므로 `handler` 사용 |
| `server/` (프로토콜 어댑터) | `main.go` (mcp-go 라이브러리 사용) | Go는 mcp-go가 서버 추상화를 제공하므로 별도 패키지 불필요 |
| `client/` (MCP 클라이언트) | 해당 없음 | prism는 서버만 구현 |
| `resources/` | `handler/docs.go` | filesystem 리소스가 4개 핸들러뿐이므로 handler에 통합 |
| `types.py` (공유 타입) | 각 패키지 내 분산 | Go 관례: 타입은 사용처 가까이 배치 |
| `job_manager.py` | `task/store.go` | 동일 관심사: 비동기 작업 수명주기 관리 |
| `errors.py` | (별도 파일 불필요) | Go는 에러를 반환값으로 처리, 커스텀 에러 타입이 적음 |

---

## 5. 주요 설계 결정 사항

### 5.1 `pipeline`을 단일 패키지로 유지하는 이유

stage1~4를 각각 별도 패키지(`pipeline/scope/`, `pipeline/specialist/` 등)로 분리하지 않은 이유:

1. **공유 타입이 많다**: `AnalysisConfig`, `Perspective`, `SeedAnalysis`, `StageResult` 등이 여러 stage에서 공유됨. 별도 패키지 시 공통 타입을 위한 추가 패키지(`pipeline/types/`)가 필요하여 오히려 복잡도 증가
2. **적절한 크기**: 병합 후 pipeline 패키지는 ~3,700 LOC (7개 소스 파일). Go 표준 라이브러리의 `net/http`(~17K LOC)와 비교하면 단일 패키지로 충분한 규모
3. **파일 단위 분리로 충분**: Go에서는 동일 패키지 내 파일 분리가 관심사 분리의 1차 수단

### 5.2 `parallel`을 별도 패키지로 분리하는 이유

1. **범용 인프라**: ParallelExecutor는 분석 도메인을 모르는 순수 동시성 유틸리티
2. **의존 방향 명확화**: pipeline → parallel 단방향 의존만 허용
3. **독립 테스트 가능**: 도메인 타입 없이 단위 테스트 가능

### 5.3 `handler` ↔ `pipeline` 경계

- **handler**: MCP request/response 직렬화, 파라미터 검증, 에러 포매팅
- **pipeline**: 비즈니스 로직 (프롬프트 구성, Claude 호출, 결과 파싱)
- 경계 원칙: handler는 `mcp.CallToolRequest`를 알지만 Claude CLI 프로세스를 직접 다루지 않음

### 5.4 `StageResult` 타입의 위치

`StageResult`는 `parallel.ParallelJob.Fn`의 반환 타입이면서 pipeline에서 정의됨.
→ `parallel` 패키지에서는 `StageResult`를 직접 참조하지 않도록 **제네릭 또는 인터페이스로 디커플링**:

```go
// parallel/executor.go
type ParallelJob struct {
    PerspectiveID string
    Fn            func(ctx context.Context) (outputPath string, err error)
}
```

pipeline 쪽에서 `StageResult`로 래핑하면 parallel은 도메인 타입에 의존하지 않게 된다.

---

## 6. 마이그레이션 시 Import 변경 범위

### 6.1 새 Import 경로

| 패키지 | Import 경로 |
|--------|------------|
| engine | `github.com/heechul/prism-mcp/internal/engine` |
| task | `github.com/heechul/prism-mcp/internal/task` |
| parallel | `github.com/heechul/prism-mcp/internal/parallel` |
| pipeline | `github.com/heechul/prism-mcp/internal/pipeline` |
| handler | `github.com/heechul/prism-mcp/internal/handler` |

### 6.2 Export 필요 항목 (현재 대문자로 시작하므로 변경 없음)

현재 모든 코드가 `package main`이므로 unexported 함수도 자유롭게 호출 가능하다.
패키지 분리 시 **cross-package 호출되는 함수/타입은 반드시 Exported(대문자)** 이어야 한다.

주요 export 필요 심볼:

| 심볼 | 현재 파일 | 목적지 패키지 | 호출 패키지 |
|------|----------|-------------|-----------|
| `TaskStore`, `NewTaskStore` | task_manager.go | task | main, handler |
| `AnalysisTask`, `TaskSnapshot` | task_manager.go | task | handler, pipeline |
| `ClaudeOptions`, `QuerySync` | claude_sdk.go | engine | pipeline, handler |
| `ParallelExecutor` | parallel.go | parallel | pipeline |
| `RunAnalysisPipeline` | analyze.go | pipeline | handler |
| `AnalysisConfig`, `StageResult` | analyze.go | pipeline | handler, pipeline |
| `Perspective`, `ReadPerspectives` | perspectives.go | pipeline | pipeline (내부) |
| `handleAnalyze` 등 | analyze.go | handler | main |
| `handleInterview` | interview.go | handler | main |
| `handleDAReview` | da_review.go | handler | main |
| `initFilesystem` | filesystem.go | handler | main |

### 6.3 변경이 필요한 unexported → exported 전환

```
runAnalysisPipeline  → RunAnalysisPipeline   (pipeline)
readAnalysisConfig   → ReadAnalysisConfig     (pipeline)
callLLM              → CallLLM                (engine)
filterEnv            → FilterEnv              (engine)
handleAnalyze        → HandleAnalyze          (handler)
handleInterview      → HandleInterview        (handler)
handleDAReview       → HandleDAReview         (handler)
handleScore          → HandleScore            (handler)
initFilesystem       → InitFilesystem         (handler)
scoreSession         → ScoreSession           (handler, 또는 handler 내부 유지 시 불필요)
```

---

## 7. 마이그레이션 순서 (권장)

순환 의존이 발생하지 않도록 리프 패키지부터 순서대로:

1. **Phase 1** — 리프 패키지 추출 (외부 의존 없음)
   - `internal/engine/` ← claude_sdk.go + llm.go
   - `internal/task/` ← task_manager.go
   - `internal/parallel/` ← parallel.go

2. **Phase 2** — 도메인 로직 추출
   - `internal/pipeline/` ← stage*.go + seed_merge.go + perspectives.go + result_collector.go + analyze.go(pipeline 부분)

3. **Phase 3** — 핸들러 추출
   - `internal/handler/` ← analyze.go(handler 부분) + interview.go + scorer.go + da_review.go + filesystem.go

4. **Phase 4** — main.go 정리
   - import 경로 변경, handler.Handle* 호출로 전환
