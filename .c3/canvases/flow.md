---
id: flow
c3-seal: c00695eda9ede8c582c26467921e9b492dba56db3c7b04e2bdcb29b59e7816bc
type: canvas
description: 'Flow: an ordered path of interactions across containers and components that realizes one outcome.'
---

domain: software
sections:
    - name: Goal
      content_type: text
      required: true
      purpose: The outcome this flow realizes and what triggers it
      min_words: 8
    - name: Steps
      content_type: table
      required: true
      purpose: Ordered hops; row order (and Seq) is the step order
      columns:
        - name: Seq
          type: text
        - name: From
          type: reference
          edge: flow_from
          targets:
            - container
            - component
        - name: To
          type: reference
          edge: flow_to
          targets:
            - container
            - component
        - name: Action
          type: text
        - name: Evidence
          type: evidence
      min_rows: 2
    - name: Failure Paths
      content_type: table
      required: false
      purpose: Where the flow can fail and the observable outcome
      columns:
        - name: At step
          type: text
        - name: Failure
          type: text
        - name: Outcome
          type: text
      min_rows: 1
reject_if:
    - A Steps row cites no From/To entity id (N.A - <reason> only for an external actor)
workorder: ""
