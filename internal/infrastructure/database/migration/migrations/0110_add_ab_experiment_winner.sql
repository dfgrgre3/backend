-- Migration: Add winner column to ABExperiment table
-- This enables declaring a winner (A or B) for A/B tests

ALTER TABLE public."ABExperiment" ADD COLUMN IF NOT EXISTS winner VARCHAR(1) DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_ab_experiment_status ON public."ABExperiment" (status);
CREATE INDEX IF NOT EXISTS idx_ab_experiment_created_at ON public."ABExperiment" (created_at DESC);