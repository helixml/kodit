(function_declaration
  name: (identifier) @function.name
  body: (block) @function.body
) @function.def

(method_declaration
  name: (field_identifier) @method.name
  body: (block) @method.body
) @method.def

(import_declaration
  (import_spec
    path: (interpreted_string_literal) @import.name
  )
) @import.statement

(identifier) @ident

(parameter_declaration
  name: (identifier) @param.name
)

(package_clause "package" (package_identifier) @name.definition.module)

;; Exclude comments from being captured
(comment) @comment