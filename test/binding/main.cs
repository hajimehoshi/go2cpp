// SPDX-License-Identifier: Apache-2.0

#pragma warning disable 649

using System;
using Go2DotNet.Test.Binding.AutoGen;

namespace Go2DotNet.Test.Binding
{
    public class Testing
    {
        // All numerics from Go are treated as double due to syscall/js.
        internal static double StaticField;

        internal static double StaticProperty
        {
            get { return staticProperty; }
            set { staticProperty = value; }
        }
        static double staticProperty;

        internal static string StaticMethod(string arg)
        {
            return arg + " and C#";
        }

        internal static Testing StaticMethodToReturnInstance(string str, double num)
        {
            return new Testing(str, num);
        }

        internal static bool Bool(bool val)
        {
            return val;
        }

        internal static object Null()
        {
            return null;
        }

        internal static double NaN()
        {
            return double.NaN;
        }

        public Testing(string str, double num)
        {
            this.str = str;
            this.num = num;
        }

        internal double InstanceField;

        internal double InstanceProperty
        {
            get { return instanceProperty; }
            set { instanceProperty = value; }
        }
        private double instanceProperty;

        internal string InstanceMethod(string arg)
        {
            return $"str: {this.str}, num: {this.num}, arg: {arg}";
        }

        internal object InvokeGo(IInvokable a, string arg)
        {
            return a.Invoke(new object[] { arg });
        }

        internal object InvokeGoWithoutArgs(IInvokable a)
        {
            return a.Invoke(null);
        }

        internal double InvokeGoAndReturnDouble(IInvokable a)
        {
            return (double)a.Invoke(null);
        }

        internal byte[] DoubleBytes(byte[] bytes)
        {
            byte[] newBytes = new byte[bytes.Length];
            for (int i = 0; i < bytes.Length; i++)
            {
                newBytes[i] = (byte)(bytes[i] * 2);
            }
            return newBytes;
        }

        internal Testing InstanceObjectProperty { get; set; }

        internal Testing Clone()
        {
            return new Testing(this.str, this.num);
        }

        internal void CopyFrom(Testing rhs)
        {
            this.str = rhs.str;
            this.num = rhs.num;
        }

        private string str;
        private double num;
    }

    class Program
    {
        static void Main(string[] args)
        {
            Go go = new Go();
            go.Run(args);
        }
    }    
}
