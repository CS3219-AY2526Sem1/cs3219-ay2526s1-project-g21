import { useState, useCallback } from 'react';
import { explainCode, DetailLevel, Language } from '@/api/ai';

export function useExplain() {
  const [loading, setLoading] = useState(false);
  const [text, setText] = useState<string>('');
  const [error, setError] = useState<string>('');

  const run = useCallback(async (args: { code: string; language: Language; detail: DetailLevel; }) => {
    setLoading(true);
    setError('');   
    setText('');
    try {
      const resp = await explainCode({
        code: args.code,
        language: args.language,
        detail_level: args.detail,
      });
      setText(resp.explanation);
    } catch (e: any) {
      setError(e?.message ?? 'Failed to generate explanation');
    } finally {
      setLoading(false);
    }
  }, []);

  return { run, loading, text, error, setText, setError };
}
