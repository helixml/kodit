(if_statement) @statement
(expression_statement) @statement
(function_definition) @statement
(return_statement) @statement

(if_statement
  condition: (_) @if_condition
  consequence: (block) @if_consequence
  alternative: (else_clause (block) @if_alternative)?
) 