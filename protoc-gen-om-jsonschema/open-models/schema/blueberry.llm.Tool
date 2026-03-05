{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "blueberry.llm.Tool",
  "type": "object",
  "description": "Tool defines a function that the model may call during chat completion.Tool",
  "properties": {
    "function": {
      "$ref": "blueberry.llm.FunctionDefinition",
      "description": "The JSON-Schema function definition."
    },
    "type": {
      "type": "string",
      "description": "The JSON-Schema type – always \"function\"."
    }
  }
}