// SPDX-License-Identifier: Apache-2.0

package main

const js = `    class JSObject
    {
        public static JSObject Undefined = new JSObjectEmpty("Undefined");
        public static JSObject Global = new JSObjectGlobal();
        
        public static object ReflectGet(object target, string key)
        {
            if (target is JSObject)
            {
                return ((JSObject)target).Get(key);
            }
            throw new Exception($"{target}.{key} not found");
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
            return "(JSObject)";
        }

        private Dictionary<string, object> values = new Dictionary<string, object>();
    }

    class JSObjectEmpty : JSObject
    {
        public JSObjectEmpty(string name)
        {
            this.name = name;
        }

        public override object Get(string key)
        {
            throw new Exception($"{this}.{key} not found");
        }

        public override string ToString()
        {
            return this.name;
        }

        string name;
    }

    class JSObjectGlobal : JSObject
    {
        private static JSObject Object = new JSObjectEmpty("Object");
        private static JSObject Array = new JSObjectEmpty("Array");
        private static JSObject FS = new JSObjectFS();

        public override object Get(string key)
        {
            switch (key)
            {
            case "Object":
                return Object;
            case "Array":
                return Array;
            case "process":
                // TODO: Implement pseudo process as wasm_exec.js does.
                return null;
            case "fs":
                // TODO: Implement pseudo fs as wasm_exec.js does.
                return FS;
            }
            throw new Exception($"{this}.{key} not found");
        }

        public override string ToString()
        {
            return "Global";
        }
    }

    class JSObjectFS : JSObject
    {
        public JSObjectFS()
        {
            this.constants = new JSObject();
            this.constants.Set("O_WRONLY", -1);
            this.constants.Set("O_RDWR", -1);
            this.constants.Set("O_CREAT", -1);
            this.constants.Set("O_TRUNC", -1);
            this.constants.Set("O_APPEND", -1);
            this.constants.Set("O_EXCL", -1);
        }

        public override object Get(string key)
        {
            switch (key)
            {
            case "constants":
                return this.constants;
            }
            throw new Exception($"{this}.{key} not found");
        }

        public override string ToString()
        {
            return "FS";
        }

        private JSObject constants;
    }`
