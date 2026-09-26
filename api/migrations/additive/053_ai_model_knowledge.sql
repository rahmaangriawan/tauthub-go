-- Migration: 053_ai_model_knowledge
-- Adds AI Model Knowledge Document and RAG semantic chunks storage
-- Strictly additive and non-destructive.

BEGIN;

CREATE TABLE IF NOT EXISTS public.ai_model_knowledge (
  id VARCHAR(36) PRIMARY KEY,
  model_id VARCHAR(120) NOT NULL UNIQUE,
  model_name VARCHAR(150),
  provider VARCHAR(60),
  reasoning_level VARCHAR(20) DEFAULT 'sedang', -- 'rendah' | 'sedang' | 'tinggi'
  summary TEXT,
  characteristics TEXT,
  suitable_for TEXT,
  unsuitable_for TEXT,
  guidelines TEXT,
  limitations TEXT,
  editorial_context TEXT,
  status VARCHAR(20) DEFAULT 'active', -- 'active' | 'draft' | 'archived'
  chunk_count INTEGER DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_model_knowledge_model ON public.ai_model_knowledge(model_id);
CREATE INDEX IF NOT EXISTS idx_ai_model_knowledge_status ON public.ai_model_knowledge(status);

CREATE TABLE IF NOT EXISTS public.ai_model_knowledge_chunks (
  id VARCHAR(36) PRIMARY KEY,
  knowledge_id VARCHAR(36) NOT NULL REFERENCES public.ai_model_knowledge(id) ON DELETE CASCADE,
  model_id VARCHAR(120) NOT NULL,
  section VARCHAR(60) NOT NULL,
  chunk_index INTEGER NOT NULL DEFAULT 0,
  content TEXT NOT NULL,
  embedding JSONB,
  priority INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_ai_model_knowledge_chunks UNIQUE (knowledge_id, section, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_ai_model_chunks_model ON public.ai_model_knowledge_chunks(model_id);
CREATE INDEX IF NOT EXISTS idx_ai_model_chunks_section ON public.ai_model_knowledge_chunks(section);

COMMIT;
