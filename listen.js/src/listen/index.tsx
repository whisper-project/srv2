/*
 * Copyright 2023-2026 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

import React, { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import App from './listen'

// @ts-ignore
const root = createRoot(document.getElementById('root'))
root.render(
    <StrictMode>
        <App />
    </StrictMode>,
)
