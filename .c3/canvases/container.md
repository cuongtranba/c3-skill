---
id: container
c3-seal: 7d979065b80eb6b07b07d8b6993ddf066ca9211d2509dd4e7df5a04a0ae7315d
type: canvas
description: 'Container: a deployable/process unit and the components it owns.'
---

domain: software
sections:
    - name: Goal
      content_type: text
      required: true
      purpose: What this container exists to do
    - name: Components
      content_type: table
      required: true
      purpose: Parts that compose this container
      columns:
        - name: ID
          type: entity_id
        - name: Name
          type: text
        - name: Category
          type: text
        - name: Status
          type: text
        - name: Goal Contribution
          type: text
    - name: Responsibilities
      content_type: text
      required: true
      purpose: What this container is accountable for
    - name: Complexity Assessment
      content_type: text
      required: false
      purpose: Known complexity and risks
    - name: Dependencies
      content_type: table
      required: false
      purpose: Components/containers this one calls or consumes at runtime (the depends_on edge)
      columns:
        - name: Depends on
          type: reference
          edge: depends_on
          targets:
            - container
        - name: Interaction
          type: text
        - name: Contract
          type: text
        - name: Evidence
          type: evidence
      min_rows: 1
reject_if: []
workorder: ""
