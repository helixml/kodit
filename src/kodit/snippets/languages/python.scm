(function_definition
  name: (identifier) @function.name
  body: (block) @function.body
) @function.def

(class_definition
  name: (identifier) @class.name
) @class.def

(import_statement
  name: (dotted_name (identifier) @import.name))

(import_from_statement
  module_name: (dotted_name (identifier) @import.from))

(identifier) @ident

(assignment
  left: (identifier) @assignment.lhs)

(parameters
  (identifier) @param.name)