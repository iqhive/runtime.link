(async function () {
  let http = async function (method, accept, path, payload) {
    try {
      let response = await fetch(path, {
        method: method,
        headers: {
          Accept: accept,
          "Content-Type": "application/json",
        },
        body: payload,
      });
      if (response.status == 204) {
        return null;
      }
      return response.json();
    } catch (error) {
      alert(error);
    }
  };
  try {
    let methods = ["GET", "POST", "PUT", "DELETE"];
    for (let method of methods) {
      let schema = await http(
        "GET",
        "application/schema+json",
        "?method=" + method,
      );

      // copy over descriptions as alpaca 'helper' options
      let fields = {};
      let definitions = [];

      for (let key in schema.properties) {
        let prop = schema.properties[key];
        fields[key] = {
          label: prop.title,
          helper: prop.description,
        };
      }

      for (let key in schema.definitions) {
        let def = schema.definitions[key];
        let properties = {};
        for (let key in def.properties) {
          let prop = def.properties[key];
          properties[key] = {
            label: prop.title,
            helper: prop.description,
          };
        }
        definitions[key] = {
          fields: properties,
        };
      }
      let data = localStorage.getItem(method + " " + location.pathname) || {};
      schema.required = null;
      let hide = null;
      if (Object.keys(schema).length === 0) {
        hide = "hidden";
      }
      console.log(fields);
      let spec = {
        data: data,
        schema: schema,
        options: {
          fields: fields,
          definitions: definitions,
          type: hide,
          form: {
            buttons: {
              submit: {
                click: async function () {
                  let body = JSON.stringify(this.getValue());
                  console.log(this);
                  console.log(this.getValue());
                  if (method === "GET") {
                    body = null;
                  }
                  let response = await http(
                    method,
                    "application/json",
                    "",
                    body,
                  );
                  if (response) {
                    $("pre").text(JSON.stringify(response, null, 2));
                    $("pre").css("display", "block");
                  } else {
                    $("pre").css("display", "none");
                  }
                },
              },
            },
          },
        },
        postRender: function (control) {
          let inputs = document.querySelectorAll("input");
          for (let index = 0; index < inputs.length; ++index) {
            let input = inputs[index];
            let save = function () {
              let value = control.getValue();
              if (value) {
                localStorage.setItem(
                  method + " " + location.pathname,
                  JSON.stringify(value),
                );
              }
            };
            window.addEventListener("click", save);
            input.addEventListener("change", save);
            input.addEventListener("blur", save);
          }
        },
      };
      $("#" + method).alpaca(spec);
    }
  } catch (err) {
    console.error(err);
  }
})();
