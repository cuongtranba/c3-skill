---
id: boundary
c3-seal: 690b7b972612c5e564a6a8661201505a6ce916d051b03ebd362f74f10b0f5077
type: canvas
description: 'Boundary: a security, network, or infra perimeter that encloses containers and components; nests via parent:.'
---

domain: software
sections:
    - name: Goal
      content_type: text
      required: true
      purpose: What this perimeter isolates and why a crossing matters
      min_words: 8
    - name: Perimeter
      content_type: table
      required: true
      purpose: The perimeter class and how a crossing is mediated
      columns:
        - name: Kind
          type: enum
          values:
            - security
            - network
            - infra
            - N.A - <reason>
        - name: Mediation
          type: text
        - name: Evidence
          type: evidence
      min_rows: 1
    - name: Members
      content_type: table
      required: true
      purpose: Containers and components enclosed by this perimeter (the encloses edge)
      columns:
        - name: Member
          type: reference
          edge: encloses
          targets:
            - container
            - component
        - name: Role
          type: text
        - name: Notes
          type: text
      min_rows: 1
    - name: Crossings
      content_type: table
      required: false
      purpose: Known paths that cross this perimeter and the control on each
      columns:
        - name: Path
          type: text
        - name: Control
          type: text
        - name: Evidence
          type: evidence
      min_rows: 1
reject_if:
    - Kind is generic instead of the perimeter class the canvas enumerates
    - Members cites nothing enclosed — an empty perimeter is a diagram, not a fact
workorder: ""
