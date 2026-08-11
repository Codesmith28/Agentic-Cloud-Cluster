import React from 'react';
import { Container, Typography } from '@mui/material';
import SubmitTask from '../components/tasks/SubmitTask';

const SubmitTaskPage = () => {
  return (
    <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
      <SubmitTask />
    </Container>
  );
};

export default SubmitTaskPage;
