// SPDX-License-Identifier: Apache-2.0

using System;
using Go2DotNet.AutoGen.github_2ecom.hajimehoshi.go2dotnet.example.helloworld;

namespace Go2DotNet.Example.HelloWorld
{
    class Program
    {
        static void Main(string[] args)
        {
            Go go = new Go();
            go.Run(args).Wait();
        }
    }    
}
