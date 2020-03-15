// SPDX-License-Identifier: Apache-2.0

using Microsoft.Xna.Framework;
using Microsoft.Xna.Framework.Graphics;
using Microsoft.Xna.Framework.Input;
using Go2DotNet.Example.MonoGame.AutoGen;

namespace Go2DotNet.Example.MonoGame
{
    static class GoGameRunner
    {
        public static void Run(IInvokable onUpdate, IInvokable onDraw)
        {
            using var g = new GoGame(onUpdate, onDraw);
            g.Run();
        }
    }

    public class GoGame : Game
    {
        GraphicsDeviceManager graphics;
        SpriteBatch spriteBatch;
        IInvokable onUpdate;
        IInvokable onDraw;

        public GoGame(IInvokable onUpdate, IInvokable onDraw)
        {
            this.onUpdate = onUpdate;
            this.onDraw = onDraw;
            graphics = new GraphicsDeviceManager(this);
            Content.RootDirectory = "Content";
            IsMouseVisible = true;
        }

        protected override void Initialize()
        {
            // TODO: Add your initialization logic here

            base.Initialize();
        }

        protected override void LoadContent()
        {
            spriteBatch = new SpriteBatch(GraphicsDevice);

            // TODO: use this.Content to load your game content here
        }

        protected override void Update(GameTime gameTime)
        {
            if (GamePad.GetState(PlayerIndex.One).Buttons.Back == ButtonState.Pressed || Keyboard.GetState().IsKeyDown(Keys.Escape))
                Exit();

            this.onUpdate.Invoke(null);

            base.Update(gameTime);
        }

        protected override void Draw(GameTime gameTime)
        {
            int v = (int)(double)this.onDraw.Invoke(null);
            GraphicsDevice.Clear(new Color(v, v, v));

            base.Draw(gameTime);
        }
    }
}
