program Umgebung;
{ GetEnv, EnvCount, EnvStr; die Umgebung kommt aus umgebung.env. }
uses Dos;
var
  I: Integer;
begin
  WriteLn(EnvCount);
  for I := 0 to EnvCount + 1 do
    WriteLn(I, ': [', EnvStr(I), ']');
  WriteLn('[', GetEnv('Name'), '] [', GetEnv('NAME'), '] [', GetEnv('pfad'), '] [',
    GetEnv('LEER'), '] [', GetEnv('FEHLT'), ']');
  WriteLn('DOS ', Lo(DosVersion), '.', Hi(DosVersion), ' ', DosVersion)
end.
