# protoc-gen-open-models
This generator will create the models for the @furo/open-models module.


Missing Features:
- OneOf
- fields which starts with _var to Xvar
- ?? Index Files
- ~~Nested Messages Support https://protobuf.dev/programming-guides/proto3/#nested~~
- ~~**FULL** Spectrum of Well Known Types https://protobuf.dev/reference/protobuf/google.protobuf/#index~~
- ~~Dependency management when needed~~ 
- Build Parameter **ApiBaseURL** with a default of "/api" ???
- Additional bindings for services (look below)
- version number of protoc-gen-open-models in generates

```
service Messaging {
  rpc GetMessage(GetMessageRequest) returns (Message) {
    option (google.api.http) = {
      get: "/v1/messages/{message_id}"
      additional_bindings {
        get: "/v1/users/{user_id}/messages/{message_id}"
      }
    };
  }
} 
```