program Voll;
{ FExpand: vollständiger Pfad ab der Wurzel wie GetDir, mit / als Trenner;
  .. über die Wurzel hinaus bleibt in der Wurzel (wie DOS). }
uses Dos;
begin
  WriteLn(FExpand('datei.txt'));
  MkDir('a');
  ChDir('a');
  WriteLn(FExpand('datei.txt'));
  WriteLn(FExpand('..\b\c.pas'));
  WriteLn(FExpand('/x/./y'));
  WriteLn(FExpand('../../z'));
  WriteLn(FExpand(''));
  WriteLn(FExpand('.'));
  ChDir('..');
  RmDir('a')
end.
