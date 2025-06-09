(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @import.name
      )
    )
  )
)

(variable_declarator
  name: (identifier) @function.name
  value: (arrow_function
    body: (statement_block) @function.body
  )
)

(class_declaration
  name: (type_identifier) @class.name
) @class.def

(method_definition
  name: (property_identifier) @function.name
  body: (statement_block) @function.body
)