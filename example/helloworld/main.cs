// SPDX-License-Identifier: Apache-2.0

using System;
using Go2DotNet.Example.HelloWorld.AutoGen;

namespace Go2DotNet.Example.HelloWorld
{
    class Program
    {
        static void Main(string[] args)
        {
            Go go = new Go();
            go.Run().Wait();
        }
    }    
}
