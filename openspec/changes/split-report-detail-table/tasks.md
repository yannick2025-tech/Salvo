# Split Report Detail Table - Implementation Tasks

## 1. Database Migration

- [x] 1.1 Create migration script to create `report_details` table
- [x] 1.2 Add migration script to copy `detail` data from `reports` to `report_details`
- [x] 1.3 Add migration script to drop `detail` column from `reports` table
- [x] 1.4 Add rollback script to restore original schema
- [x] 1.5 Create indexes on `reports` table (scene_id, run_id, status, started_at)
- [ ] 1.6 Write unit tests for migration scripts
- [ ] 1.7 Test migration on development database and verify data integrity

## 2. Backend Model and Repository Changes

- [x] 2.1 Remove `Detail` field from `Report` model in `internal/store/model/model.go`
- [x] 2.2 Create new `ReportDetail` model struct in `internal/store/model/model.go`
- [x] 2.3 Update `ReportRepo` interface to support detail queries (if needed)
- [x] 2.4 Modify `List()` method in `internal/store/sqlite/sqlite.go` to query only `reports` table
- [x] 2.5 Modify `GetByID()` method to JOIN `report_details` table
- [x] 2.6 Add `CreateReportDetail()` method to insert into `report_details` table
- [x] 2.7 Update `CreateReport()` to insert into both tables
- [x] 2.8 Update `DeleteReport()` to handle cascade deletion (or manual deletion)
- [x] 2.9 Write unit tests for repository changes

## 3. Backend API Handler Changes

- [x] 3.1 Create `ReportListItemDTO` struct in `internal/api/dto/dto.go` (without `detail` field)
- [x] 3.2 Add `toReportListItemDTO()` conversion function in `internal/api/handler.go`
- [x] 3.3 Modify `ListReports` handler to return `ReportListItemDTO` instead of `ReportDTO`
- [x] 3.4 Verify `GetReport` handler works with JOIN query (should already work)
- [x] 3.5 Verify `ExportReport` handler works with JOIN query (should already work)
- [x] 3.6 Write unit tests for API handlers
- [ ] 3.7 Test API endpoints with HTTP client (Postman/curl)

## 4. Frontend API Client Changes

- [x] 4.1 Add `ReportListItemDTO` type definition in `web/app/src/types/index.ts`
- [x] 4.2 Add `listReports()` function return type for `ReportListItemDTO[]`
- [x] 4.3 Verify `getReport()` function type definition matches `ReportDTO` (with detail)
- [x] 4.4 Test API client functions with backend

## 5. Frontend List Page Changes

- [x] 5.1 Update `ReportsPage.vue` to use `listReports()` with new return type
- [x] 5.2 Add memory cache for report details (Map<string, ReportDTO>)
- [x] 5.3 Implement smart preloading logic (preload first 5 reports)
- [x] 5.4 Add cache hit/miss tracking for analytics
- [x] 5.5 Update `viewReport()` function to use cached detail when available
- [x] 5.6 Add loading indicator for non-cached reports
- [x] 5.7 Clear cache on component unmount (using `onUnmounted` hook)
- [x] 5.8 Test list page rendering with new API
- [x] 5.9 Test preloading behavior and cache management
- [x] 5.10 Test user navigation to report detail page

## 6. Backend Tests

- [x] 6.1 Write migration integration tests (verify schema changes)
- [x] 6.2 Write repository tests for `List()` performance
- [x] 6.3 Write repository tests for `GetByID()` with JOIN
- [x] 6.4 Write API handler tests for `ListReports` endpoint
- [x] 6.5 Write API handler tests for `GetReport` endpoint
- [x] 6.6 Run all existing tests to ensure no regressions

## 7. Frontend Tests

- [ ] 7.1 Write unit tests for preloading logic
- [ ] 7.2 Write unit tests for cache management
- [ ] 7.3 Write integration tests for list page with new API
- [ ] 7.4 Test E2E flow: list load → preload → view detail
- [ ] 7.5 Test E2E flow: list load → click non-preloaded report → view detail

## 8. Performance Testing

- [ ] 8.1 Benchmark `ListReports` API response time (target: < 1 second)
- [ ] 8.2 Benchmark `GetReport` API response time (target: < 100ms)
- [ ] 8.3 Measure list response payload size (target: < 50KB for 50 reports)
- [ ] 8.4 Compare performance before and after optimization
- [ ] 8.5 Test with realistic data volume (100+ reports)

## 9. Documentation and Finalization

- [ ] 9.1 Update API documentation for `ListReports` endpoint
- [ ] 9.2 Update database schema documentation
- [ ] 9.3 Add migration guide for developers
- [ ] 9.4 Document preloading behavior in frontend code comments
- [ ] 9.5 Final code review and cleanup
- [ ] 9.6 Run full test suite before deployment
- [ ] 9.7 Create deployment checklist