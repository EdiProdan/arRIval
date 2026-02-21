{
  "openapi": "3.0.1",
  "info": {
    "title": "OpenApiService",
    "version": "1.0"
  },
  "paths": {
    "/api/open/v1/voznired/autobus": {
      "get": {
        "tags": [
          "Autobus"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "gbr",
            "in": "query",
            "schema": {
              "type": "integer",
              "format": "int32"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/RacStatusPublicListResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/RacStatusPublicListResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/RacStatusPublicListResponse"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/voznired/autobusi": {
      "get": {
        "tags": [
          "Autobus"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/RacStatusPublicListResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/RacStatusPublicListResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/RacStatusPublicListResponse"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/token/login": {
      "get": {
        "tags": [
          "Token"
        ],
        "parameters": [
          {
            "name": "Username",
            "in": "header",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "Password",
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "type": "string"
                }
              },
              "application/json": {
                "schema": {
                  "type": "string"
                }
              },
              "text/json": {
                "schema": {
                  "type": "string"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/token/refresh": {
      "get": {
        "tags": [
          "Token"
        ],
        "parameters": [
          {
            "name": "Username",
            "in": "header",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "type": "string"
                }
              },
              "application/json": {
                "schema": {
                  "type": "string"
                }
              },
              "text/json": {
                "schema": {
                  "type": "string"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/voznired/linije": {
      "get": {
        "tags": [
          "VozniRed"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/StringModelLinijaDictionaryResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StringModelLinijaDictionaryResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/StringModelLinijaDictionaryResponse"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/voznired/stanice": {
      "get": {
        "tags": [
          "VozniRed"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/Int32ModelStanicaDictionaryResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/Int32ModelStanicaDictionaryResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/Int32ModelStanicaDictionaryResponse"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/voznired/polasci": {
      "get": {
        "tags": [
          "VozniRed"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/StringModelLinijaPolazakStanicaDictionaryResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/StringModelLinijaPolazakStanicaDictionaryResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/StringModelLinijaPolazakStanicaDictionaryResponse"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/voznired/polasciStanica": {
      "get": {
        "tags": [
          "VozniRed"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "stanicaId",
            "in": "query",
            "schema": {
              "type": "integer",
              "format": "int32"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/ModelStanicaResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ModelStanicaResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/ModelStanicaResponse"
                }
              }
            }
          }
        }
      }
    },
    "/api/open/v1/voznired/polasciLinija": {
      "get": {
        "tags": [
          "VozniRed"
        ],
        "parameters": [
          {
            "name": "token",
            "in": "header",
            "schema": {
              "type": "string"
            }
          },
          {
            "name": "uniqueLinijaId",
            "in": "query",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success",
            "content": {
              "text/plain": {
                "schema": {
                  "$ref": "#/components/schemas/ModelLinijaResponse"
                }
              },
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/ModelLinijaResponse"
                }
              },
              "text/json": {
                "schema": {
                  "$ref": "#/components/schemas/ModelLinijaResponse"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "Int32ModelStanicaDictionaryResponse": {
        "type": "object",
        "properties": {
          "msg": {
            "type": "string",
            "nullable": true
          },
          "res": {
            "type": "object",
            "additionalProperties": {
              "$ref": "#/components/schemas/ModelStanica"
            },
            "nullable": true
          },
          "err": {
            "type": "boolean"
          }
        },
        "additionalProperties": false
      },
      "ModelLinija": {
        "type": "object",
        "properties": {
          "id": {
            "type": "integer",
            "format": "int32"
          },
          "brojLinije": {
            "type": "string",
            "nullable": true
          },
          "smjerId": {
            "type": "integer",
            "format": "int32"
          },
          "smjerNaziv": {
            "type": "string",
            "nullable": true
          },
          "varijantaId": {
            "type": "integer",
            "format": "int32"
          },
          "naziv": {
            "type": "string",
            "nullable": true
          },
          "polazakList": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/ModelPolazak"
            },
            "nullable": true
          }
        },
        "additionalProperties": false
      },
      "ModelLinijaPolazakStanica": {
        "type": "object",
        "properties": {
          "id": {
            "type": "integer",
            "format": "int32"
          },
          "brojLinije": {
            "type": "string",
            "nullable": true
          },
          "smjerId": {
            "type": "integer",
            "format": "int32"
          },
          "smjerNaziv": {
            "type": "string",
            "nullable": true
          },
          "varijantaId": {
            "type": "integer",
            "format": "int32"
          },
          "naziv": {
            "type": "string",
            "nullable": true
          },
          "polazakList": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/ModelPolazakStanica"
            },
            "nullable": true
          }
        },
        "additionalProperties": false
      },
      "ModelLinijaResponse": {
        "type": "object",
        "properties": {
          "msg": {
            "type": "string",
            "nullable": true
          },
          "res": {
            "$ref": "#/components/schemas/ModelLinija"
          },
          "err": {
            "type": "boolean"
          }
        },
        "additionalProperties": false
      },
      "ModelPolazak": {
        "type": "object",
        "properties": {
          "stanicaId": {
            "type": "integer",
            "format": "int32"
          },
          "voznjaId": {
            "type": "integer",
            "format": "int32"
          },
          "voznjaBusId": {
            "type": "integer",
            "format": "int32"
          },
          "voznjaStanicaId": {
            "type": "integer",
            "format": "int32"
          },
          "linijaId": {
            "type": "integer",
            "format": "int32"
          },
          "uniqueLinijaId": {
            "type": "string",
            "nullable": true
          },
          "polazak": {
            "type": "string",
            "format": "date-time",
            "nullable": true
          },
          "dolazak": {
            "type": "string",
            "format": "date-time",
            "nullable": true
          }
        },
        "additionalProperties": false
      },
      "ModelPolazakStanica": {
        "type": "object",
        "properties": {
          "stanicaId": {
            "type": "integer",
            "format": "int32"
          },
          "voznjaId": {
            "type": "integer",
            "format": "int32"
          },
          "voznjaBusId": {
            "type": "integer",
            "format": "int32"
          },
          "voznjaStanicaId": {
            "type": "integer",
            "format": "int32"
          },
          "linijaId": {
            "type": "integer",
            "format": "int32"
          },
          "uniqueLinijaId": {
            "type": "string",
            "nullable": true
          },
          "stanica": {
            "$ref": "#/components/schemas/ModelStanica"
          },
          "polazak": {
            "type": "string",
            "format": "date-time",
            "nullable": true
          },
          "dolazak": {
            "type": "string",
            "format": "date-time",
            "nullable": true
          }
        },
        "additionalProperties": false
      },
      "ModelStanica": {
        "type": "object",
        "properties": {
          "id": {
            "type": "integer",
            "format": "int32"
          },
          "naziv": {
            "type": "string",
            "nullable": true
          },
          "nazivKratki": {
            "type": "string",
            "nullable": true
          },
          "gpsX": {
            "type": "number",
            "format": "double"
          },
          "gpsY": {
            "type": "number",
            "format": "double"
          },
          "smjer": {
            "type": "string",
            "nullable": true
          },
          "smjerId": {
            "type": "integer",
            "format": "int32",
            "nullable": true
          },
          "stanicaIdSuprotniSmjer": {
            "type": "integer",
            "format": "int32",
            "nullable": true
          },
          "polazakList": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/ModelPolazak"
            },
            "nullable": true
          }
        },
        "additionalProperties": false
      },
      "ModelStanicaResponse": {
        "type": "object",
        "properties": {
          "msg": {
            "type": "string",
            "nullable": true
          },
          "res": {
            "$ref": "#/components/schemas/ModelStanica"
          },
          "err": {
            "type": "boolean"
          }
        },
        "additionalProperties": false
      },
      "RacStatusPublic": {
        "type": "object",
        "properties": {
          "gbr": {
            "type": "integer",
            "format": "int32",
            "nullable": true
          },
          "lon": {
            "type": "number",
            "format": "double",
            "nullable": true
          },
          "lat": {
            "type": "number",
            "format": "double",
            "nullable": true
          },
          "voznjaId": {
            "type": "integer",
            "format": "int32",
            "nullable": true
          },
          "voznjaBusId": {
            "type": "integer",
            "format": "int32",
            "nullable": true
          }
        },
        "additionalProperties": false
      },
      "RacStatusPublicListResponse": {
        "type": "object",
        "properties": {
          "msg": {
            "type": "string",
            "nullable": true
          },
          "res": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/RacStatusPublic"
            },
            "nullable": true
          },
          "err": {
            "type": "boolean"
          }
        },
        "additionalProperties": false
      },
      "StringModelLinijaDictionaryResponse": {
        "type": "object",
        "properties": {
          "msg": {
            "type": "string",
            "nullable": true
          },
          "res": {
            "type": "object",
            "additionalProperties": {
              "$ref": "#/components/schemas/ModelLinija"
            },
            "nullable": true
          },
          "err": {
            "type": "boolean"
          }
        },
        "additionalProperties": false
      },
      "StringModelLinijaPolazakStanicaDictionaryResponse": {
        "type": "object",
        "properties": {
          "msg": {
            "type": "string",
            "nullable": true
          },
          "res": {
            "type": "object",
            "additionalProperties": {
              "$ref": "#/components/schemas/ModelLinijaPolazakStanica"
            },
            "nullable": true
          },
          "err": {
            "type": "boolean"
          }
        },
        "additionalProperties": false
      }
    }
  }
}