# Data Model Document

**Project:** specs
**Generated At:** 2026-03-11T16:26:06.910654Z
**Schema Version:** 1.0
**Root Path:** `/Users/dshills/Development/projects/witness/specs`

---

## Overview

This document describes the data model for the **specs** project. It was produced by static analysis of the project source at the path noted above.

> ⚠️ **Notice:** No datastores, components, APIs, jobs, integrations, or configuration sources were detected during analysis. All sections below reflect the absence of discoverable data. Fields and relationships that cannot be derived from the fact model are marked **UNKNOWN**.

---

## 1. Detected Datastores

| # | Datastore Name | Type | Host | Port | Database | Schema Confidence |
|---|---------------|------|------|------|----------|-------------------|
| — | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | 0% |

No datastores were detected in the fact model. The `datastores` array is empty. No connection strings, ORM models, migration files, or query patterns were identified during analysis.

---

## 2. Inferred Schemas

Because no datastores were detected, no table or collection schemas can be inferred.

> If datastores are added to the project and re-analyzed, this section will be populated with:
> - Table / collection names
> - Column / field names and inferred types
> - Primary and foreign key relationships
> - Any PII-flagged fields

---

## 3. Entities & Fields

| Entity | Field | Inferred Type | Nullable | PII | Notes |
|--------|-------|---------------|----------|-----|-------|
| UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | No entities detected |

---

## 4. PII-Flagged Fields

No PII-flagged fields were identified during analysis.

> ⚠️ **PII Detection Status:** The security block reports a confidence score of **0** and `inferred: false`, meaning no security or PII analysis was completed. This does **not** guarantee the absence of PII in the codebase — it indicates that no source files were available for analysis (`source_files: null`).

---

## 5. Entity Relationship Diagram

The diagram below reflects the current (empty) state of the detected data model. No entities or relationships could be derived.

```mermaid
erDiagram
    UNKNOWN {
        string field_name UNKNOWN "No schema detected"
    }
```

> Once datastores and entities are detected, this diagram will be updated to reflect actual tables, fields, and foreign-key relationships.

---

## 6. Security & Confidence Summary

| Property | Value |
|----------|-------|
| Source Files Analyzed | `null` (none) |
| Line Ranges Analyzed | `null` (none) |
| Confidence Score | `0` |
| Inferred | `false` |

The security analysis block contains no data. A confidence score of **0** means no conclusions about security posture, PII exposure, or sensitive data handling can be drawn from this fact model.

---

## 7. Recommendations

Given the empty fact model, the following steps are recommended before relying on this document:

1. **Verify project root path** — Confirm that `/Users/dshills/Development/projects/witness/specs` contains analyzable source files.
2. **Re-run analysis** — Ensure the analysis tool has read access to all relevant source files, ORM definitions, migration scripts, and configuration files.
3. **Check language detection** — The `languages` array is empty, which may indicate the analyzer could not identify the programming language(s) in use.
4. **Review security configuration** — Enable PII scanning and security analysis so that sensitive fields can be flagged in future runs.
5. **Populate datastores** — If the project uses a database, ensure connection definitions or schema files are present and accessible to the analyzer.

---

## 8. Document Metadata

| Property | Value |
|----------|-------|
| Document Version | 1.0 |
| Fact Model Schema Version | 1.0 |
| Analysis Timestamp | 2026-03-11T16:26:06.910654Z |
| Components Detected | 0 |
| APIs Detected | 0 |
| Datastores Detected | 0 |
| Jobs Detected | 0 |
| Integrations Detected | 0 |
| Config Sources Detected | 0 |

---

*This document was auto-generated from a static analysis fact model. All **UNKNOWN** values indicate fields that could not be derived from the available data. Manual review is required to validate or supplement this document.*