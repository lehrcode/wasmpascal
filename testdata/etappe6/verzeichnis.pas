program Verzeichnis;
var
  s: string;
  f: Text;
begin
  GetDir(0, s); WriteLn(s);
  MkDir('daten');
  ChDir('daten');
  GetDir(0, s); WriteLn(s);
  Assign(f, 'drin.txt'); Rewrite(f); WriteLn(f, 'im Ordner'); Close(f);
  ChDir('/');
  GetDir(0, s); WriteLn(s);
  Assign(f, 'daten/drin.txt'); Reset(f); ReadLn(f, s); Close(f); WriteLn(s);
  Erase(f);
  RmDir('daten');
  {$I-} ChDir('daten'); {$I+}
  WriteLn(IOResult)
end.
