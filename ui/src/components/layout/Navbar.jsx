// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from 'react';
import { AppBar, Toolbar, Typography, IconButton, Box } from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import CloudIcon from '@mui/icons-material/Cloud';

const Navbar = ({ onMenuClick }) => {
  return (
    <AppBar position="fixed" sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}>
      <Toolbar>
        <IconButton
          color="inherit"
          edge="start"
          onClick={onMenuClick}
          sx={{ mr: 2 }}
        >
          <MenuIcon />
        </IconButton>
        
        <CloudIcon sx={{ mr: 1 }} />
        <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
          CloudAI - Distributed Task Scheduler
        </Typography>
        
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <Typography variant="body2">
            Master: localhost:8080
          </Typography>
        </Box>
      </Toolbar>
    </AppBar>
  );
};

export default Navbar;
