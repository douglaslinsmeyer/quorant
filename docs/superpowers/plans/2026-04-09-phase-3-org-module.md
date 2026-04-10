# Phase 3: Org Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Full Org module with CRUD for organizations, memberships, units, properties, vendors, amenities, and registrations — matching all API endpoints in architecture doc Section 4.

**Architecture:** Repository pattern with PostgreSQL. Each entity group has domain types, a repository interface, PostgreSQL implementation, service, and HTTP handlers. Routes are registered on the shared mux with auth + tenant context middleware.

**Tech Stack:** Go 1.23+, pgx/v5, http.ServeMux, testify

---

## Tasks

### Task 1: Unit & Property Tables Migration
Create `backend/migrations/20260409000006_units.sql` with: units, properties, unit_memberships, unit_ownership_history tables + indexes from arch doc.

### Task 2: Vendor & Amenity Tables Migration
Create `backend/migrations/20260409000007_vendors_amenities.sql` with: vendors, vendor_assignments, amenities, amenity_reservations, unit_registration_types, unit_registrations tables + indexes.

### Task 3: Org Domain Types
Create `backend/internal/org/domain.go` with Go structs for: Organization, Membership (reuse from iam or define org-scoped view), Unit, Property, UnitMembership, UnitOwnershipHistory, Vendor, VendorAssignment, Amenity, AmenityReservation, RegistrationType, Registration. Plus request/response types for CRUD operations.

### Task 4: Org Repository — Organizations
Create `backend/internal/org/org_repository.go` (interface) and `backend/internal/org/org_postgres.go` (impl). CRUD operations: Create, FindByID, FindBySlug, List (by user access), Update, SoftDelete. ltree path management. Integration tests.

### Task 5: Org Repository — Memberships
Create `backend/internal/org/membership_repository.go` and `backend/internal/org/membership_postgres.go`. CRUD: Create (invite), List by org, Update (role/status), SoftDelete. Integration tests.

### Task 6: Org Repository — Units
Create `backend/internal/org/unit_repository.go` and `backend/internal/org/unit_postgres.go`. CRUD: Create, List by org, FindByID, Update, SoftDelete. Property management (Get/Set). Unit membership CRUD. Ownership transfer. Integration tests.

### Task 7: Org Service
Create `backend/internal/org/service.go` with OrgService orchestrating organization, membership, and unit operations. Business logic: slug generation, ltree path computation, firm↔HOA lifecycle (connect/disconnect), ownership transfer workflow.

### Task 8: Org Handlers — Organizations
Create `backend/internal/org/handler_org.go` with handlers for: POST/GET/GET:id/PATCH/DELETE organizations, GET children, POST/DELETE management, GET management history.

### Task 9: Org Handlers — Memberships
Create `backend/internal/org/handler_membership.go` with handlers for: POST/GET/GET:id/PATCH/DELETE memberships.

### Task 10: Org Handlers — Units
Create `backend/internal/org/handler_unit.go` with handlers for: POST/GET/GET:id/PATCH/DELETE units, GET/PUT property, POST/GET/PATCH/DELETE unit memberships, POST transfer, GET ownership-history.

### Task 11: Org Routes & Wiring
Create `backend/internal/org/routes.go` registering all routes. Wire into `cmd/quorant-api/main.go`.

### Task 12: Vendor & Amenity Handlers (deferred if time)
Vendor CRUD, vendor assignments, amenity CRUD, reservations, registration types, registrations.

---

## Verification
1. All unit tests pass
2. Integration tests pass against Docker PG
3. Full CRUD cycle works for organizations, memberships, units via curl
4. ltree hierarchy queries work
5. Firm↔HOA connect/disconnect lifecycle works
