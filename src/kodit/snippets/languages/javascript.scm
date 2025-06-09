(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @import.name
      )
    )
  )
)

(function_declaration
  name: (identifier) @function.name
  body: (statement_block) @function.body
)

(class_declaration
  name: (identifier) @class.name
  body: (class_body) @class.body
) @class.def

(method_definition
  name: (property_identifier) @function.name
  body: (statement_block) @function.body
)