(async function () {
  let link = function (obj, defs) {
    Object.keys(obj).forEach(function (prop, index, array) {
      var def = obj[prop];
      if (def.$ref) {
        if (def.type == "object") {
          var ref = def.$ref.replace(/^#\/$defs\//, "");
          obj[prop] = defs[ref];
        } else {
          delete def.$ref;
        }
      } else if (typeof def === "object") {
        link(def, defs);
      }
    });
  };
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
      return response.json();
    } catch (error) {
      alert(error);
    }
  };
  try {
    let schema = await http("GET", "application/schema+json", "");

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
    let data = localStorage.getItem(location.pathname) || {};
    let spec = {
      data: data,
      schema: schema,
      options: {
        fields: fields,
        definitions: definitions,
        form: {
          buttons: {
            submit: {
              click: async function () {
                let response = await http(
                  "POST",
                  "application/json",
                  "",
                  JSON.stringify(this.getValue()),
                );
                $("pre").text(JSON.stringify(response, null, 2));
                $("pre").css("display", "block");
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
            localStorage.setItem(
              location.pathname,
              JSON.stringify(control.getValue()),
            );
          };
          window.addEventListener("click", save);
          input.addEventListener("change", save);
          input.addEventListener("blur", save);
        }
      },
    };
    console.log(spec);
    $("form").alpaca(spec);
  } catch (err) {
    console.error(err);
  }
  try {
    let resource = await http("GET", "application/json", "");
    $("pre").text(JSON.stringify(resource, null, 2));
    $("pre").css("display", "block");
  } catch (err) {
    console.error(err);
  }
})();
