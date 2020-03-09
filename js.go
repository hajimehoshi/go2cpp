// SPDX-License-Identifier: Apache-2.0

package main

const js = `    class JSObject
    {
        public static JSObject Undefined = new JSObject("undefined");
        public static JSObject Global;

        static JSObject()
        {
            JSObject obj = new JSObject("Object");
            JSObject arr = new JSObject("Array");

            JSObject fs = new JSObject("fs", new Dictionary<string, object>()
            {
                {"constants", new JSObject(new Dictionary<string, object>()
                    {
                        {"O_WRONLY", -1},
                        {"O_RDWR", -1},
                        {"O_CREAT", -1},
                        {"O_TRUNC", -1},
                        {"O_APPEND", -1},
                        {"O_EXCL", -1},
                    })},
            });

            Global = new JSObject("global", new Dictionary<string, object>()
            {
                {"Object", obj},
                {"Array", arr},
                {"process", null},
                {"fs", fs},
                {"Uint8Array", null},
            });
        }
        
        public static object ReflectGet(object target, string key)
        {
            if (target == Undefined)
            {
                throw new Exception("undefined.{key} not found");
            }
            if (target is JSObject)
            {
                return ((JSObject)target).Get(key);
            }
            throw new Exception($"{target}.{key} not found");
        }

        public JSObject(Dictionary<string, object> values)
            : this("(JSObject)", values)
        {
        }

        public JSObject(string name)
            : this(name, new Dictionary<string, object>())
        {
        }

        public JSObject(string name, Dictionary<string, object> values)
        {
            this.name = name;
            this.values = values;
        }

        public virtual object Get(string key)
        {
            if (this.values.ContainsKey(key))
            {
                return this.values[key];
            }
            throw new Exception($"{this}.{key} not found");
        }

        public void Set(string key, object value)
        {
            this.values[key] = value;
        }

        public override string ToString()
        {
            return this.name;
        }

        private Dictionary<string, object> values;
        private string name;
    }`
