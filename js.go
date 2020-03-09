// SPDX-License-Identifier: Apache-2.0

package main

const js = `    abstract class JSObject
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

        public abstract object Get(string key);
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
        public override object Get(string key)
        {
            throw new Exception($"{this}.{key} not found");
        }

        public override string ToString()
        {
            return "FS";
        }
    }`
