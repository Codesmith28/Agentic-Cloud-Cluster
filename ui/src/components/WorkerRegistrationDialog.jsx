import React, { useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Grid,
  Alert,
  CircularProgress,
} from '@mui/material';
import { workersAPI } from '../api/workers';

const WorkerRegistrationDialog = ({ open, onClose, onSuccess }) => {
  const [formData, setFormData] = useState({
    worker_id: '',
    worker_ip: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const handleChange = (e) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const data = {
        worker_id: formData.worker_id.trim(),
        worker_ip: formData.worker_ip.trim(),
      };

      await workersAPI.registerWorker(data);
      onSuccess();
      handleClose();
    } catch (err) {
      setError(err.response?.data?.error || err.message || 'Failed to register worker');
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setFormData({
      worker_id: '',
      worker_ip: '',
    });
    setError('');
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <form onSubmit={handleSubmit}>
        <DialogTitle>Register New Worker</DialogTitle>
        <DialogContent>
          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}
          <Grid container spacing={2} sx={{ mt: 1 }}>
            <Grid item xs={12}>
              <TextField
                name="worker_id"
                label="Worker ID"
                value={formData.worker_id}
                onChange={handleChange}
                fullWidth
                required
                placeholder="e.g., worker-1"
                helperText="Unique identifier for this worker"
              />
            </Grid>
            <Grid item xs={12}>
              <TextField
                name="worker_ip"
                label="Worker IP:Port"
                value={formData.worker_ip}
                onChange={handleChange}
                fullWidth
                required
                placeholder="e.g., 192.168.1.100:50052"
                helperText="Address where the worker is listening (resources will be auto-detected when worker connects)"
              />
            </Grid>
          </Grid>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose} disabled={loading}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="contained"
            disabled={loading}
            startIcon={loading && <CircularProgress size={20} />}
          >
            {loading ? 'Registering...' : 'Register Worker'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
};

export default WorkerRegistrationDialog;
