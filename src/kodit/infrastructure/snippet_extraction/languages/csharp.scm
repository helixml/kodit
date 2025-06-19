(method_declaration
  name: (identifier) @function.name
  body: (block) @function.body
) @function.def

(class_declaration
  name: (identifier) @class.name
) @class.def

(using_directive) @import.name

(identifier) @ident
