# Billing Dashboard

Customer self-service portal for monitoring usage, managing API keys, and viewing invoices.

## Features

- **Usage Dashboard**: Real-time usage monitoring with charts and metrics
- **API Key Management**: Create, view, and revoke API keys
- **Invoice Management**: View and download invoices
- **JWT Authentication**: Secure login with token-based authentication
- **Responsive Design**: Works on desktop and mobile devices

## Tech Stack

- **React 18** with TypeScript
- **Vite** - Fast build tool
- **Tailwind CSS** - Utility-first styling
- **Recharts** - Data visualization
- **React Router** - Client-side routing
- **Axios** - HTTP client
- **Lucide React** - Icon library

## Getting Started

### Prerequisites

- Node.js 18+ and npm/yarn
- Dashboard API running on `http://localhost:8080`

### Installation

1. Install dependencies:

```bash
npm install
```

2. Configure environment variables:

```bash
cp .env.example .env
```

Edit `.env` and set:

```
VITE_API_URL=http://localhost:8080
```

3. Run development server:

```bash
npm run dev
```

The dashboard will be available at `http://localhost:3000`

### Build for Production

```bash
npm run build
```

The production build will be in the `dist/` directory.

## Project Structure

```
dashboard/
├── src/
│   ├── api/
│   │   └── client.ts          # API client wrapper
│   ├── components/
│   │   ├── UsageChart.tsx     # Line chart for usage trends
│   │   └── RateLimitGauge.tsx # Progress bar for rate limits
│   ├── pages/
│   │   ├── Login.tsx          # Login page
│   │   ├── UsageDashboard.tsx # Main dashboard
│   │   ├── APIKeys.tsx        # API key management
│   │   └── Invoices.tsx       # Invoice list
│   ├── types/
│   │   └── index.ts           # TypeScript type definitions
│   ├── App.tsx                # Main app component with routing
│   ├── main.tsx               # Entry point
│   └── index.css              # Global styles
├── index.html                 # HTML template
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```

## Features

### 1. Usage Dashboard

- Real-time usage for current day
- Historical usage charts (7/30/90 days)
- Cost breakdown by metric
- Rate limit gauges with warnings

### 2. API Key Management

- Create new API keys with optional expiration
- View all API keys with status
- Copy keys to clipboard
- Revoke active keys
- Track last usage

### 3. Invoice Management

- Paginated invoice list
- Download PDF invoices
- View billing period and amounts
- Status badges (paid, pending, etc.)

## Authentication

The dashboard uses JWT-based authentication:

1. User logs in with email/password
2. API returns JWT token
3. Token stored in localStorage
4. Token sent with all API requests in Authorization header
5. Auto-redirect to login on token expiration

## API Integration

All API calls go through the `apiClient` in `src/api/client.ts`:

```typescript
import { apiClient } from "./api/client";

// Login
const response = await apiClient.login({ email, password });

// Get usage
const usage = await apiClient.getCurrentUsage();

// Create API key
const key = await apiClient.createAPIKey({ name: "Production" });
```

## Environment Variables

| Variable       | Description            | Default                 |
| -------------- | ---------------------- | ----------------------- |
| `VITE_API_URL` | Dashboard API base URL | `http://localhost:8080` |

## Development

### Running Tests

```bash
npm test
```

### Linting

```bash
npm run lint
```

### Type Checking

```bash
tsc --noEmit
```

## Deployment

### Option 1: Static Hosting (Vercel, Netlify)

```bash
npm run build
# Deploy dist/ folder
```

### Option 2: Docker

```dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### Option 3: Serve with Node

```bash
npm install -g serve
serve -s dist -l 3000
```

## Customization

### Styling

Edit `tailwind.config.js` to customize colors, fonts, etc.

### API Endpoints

Modify `src/api/client.ts` to add/change endpoints.

### Add New Pages

1. Create component in `src/pages/`
2. Add route in `src/App.tsx`
3. Add navigation link in sidebar

## Troubleshooting

**CORS Issues:**

- Ensure API has CORS enabled for `http://localhost:3000`
- Check `VITE_API_URL` environment variable

**Authentication Errors:**

- Verify JWT_SECRET matches between frontend and backend
- Check token expiration time

**Build Errors:**

- Clear node_modules and reinstall: `rm -rf node_modules && npm install`
- Check Node.js version: `node --version` (18+ required)

## License

MIT License
