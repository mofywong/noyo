# Noyo Frontend

This is the Vue 3 implementation of the Noyo frontend.

## Setup

1. Install dependencies:
   ```bash
   npm install
   ```

2. Start development server:
   ```bash
   npm run dev
   ```
   Access at http://localhost:5173

## Production Build

To build for production and replace the existing static files:

1. Build:
   ```bash
   npm run build
   ```

2. Copy files:
   Copy the contents of `dist/` to `../resource/public/`.
   
   *Note: This will overwrite the existing `index.html`.*

## Features

- Vue 3 + Vite
- Bootstrap 5
- Internationalization (en/zh)
- API Integration
