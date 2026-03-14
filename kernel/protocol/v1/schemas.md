# Schemas

Schemas define how accepted claims are validated and interpreted.

## Purpose

A schema is a first-class structural object in the registry model.
It gives meaning to claims over time.

## Core Rules

- schemas should have stable identity
- schema versions should be explicit
- claims should resolve through a schema/version reference
- published schema versions should not be silently rewritten in place

## Why Schemas Are Objects

Schemas behave more like durable structural entities than incidental metadata.

They need:

- identity
- versioning
- publication rules
- permissions
- references from claims and materialization logic

That is why schemas should be treated as schema objects inside the registry
model.

## Schema Identity

The model should distinguish between:

- schema object identity
- schema version identity

That lets the system say:

- this claim uses the task-title schema
- this accepted interpretation depends on version 3 of that schema

## Relationship To Claims

Claims should not rely on floating interpretation.

A claim should point at an explicit schema/version rule so replay stays
trustworthy.

## Relationship To Materialization

The materializer should resolve claims through explicit schema/version rules.

That keeps:

- validation deterministic
- interpretation replayable
- evolution visible instead of implicit
