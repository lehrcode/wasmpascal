program Verzeichnisse;
{ ChDir, MkDir, RmDir und GetDir mit normalisierten Pfaden und den
  E/A-Fehlern 2, 3 und 5. IOResult immer zuerst in N lesen: Solange ein
  Fehler ansteht, ruht jede E/A, auch WriteLn. }
var
  S: string;
  F: Text;
  N: Integer;
begin
  MkDir('a');
  ChDir('a');
  GetDir(0, S); WriteLn('in a: ', S);
  MkDir('b');
  ChDir('b/../b/./');
  GetDir(0, S); WriteLn('in a/b: ', S);
  ChDir('..');
  GetDir(0, S); WriteLn('zurueck: ', S);
  ChDir('..');
  GetDir(0, S); WriteLn('Wurzel: ', S);
  {$I-} ChDir('..'); {$I+} N := IOResult;
  GetDir(0, S); WriteLn('ueber die Wurzel: ', N, ' ', S);
  {$I-} ChDir('fehlt'); {$I+} N := IOResult;
  WriteLn('ChDir fehlt: ', N);
  {$I-} MkDir('a'); {$I+} N := IOResult;
  WriteLn('MkDir vorhanden: ', N);
  {$I-} RmDir('a'); {$I+} N := IOResult;
  WriteLn('RmDir nicht leer: ', N);
  {$I-} RmDir('fehlt'); {$I+} N := IOResult;
  WriteLn('RmDir fehlt: ', N);
  { relativer Pfad nach ChDir }
  ChDir('a\b');
  Assign(F, 'notiz.txt'); Rewrite(F); WriteLn(F, 'in a/b'); Close(F);
  ChDir('\');
  Assign(F, 'a/b/notiz.txt'); Reset(F); ReadLn(F, S); Close(F);
  WriteLn('gelesen: ', S);
  { Pfade ausserhalb der Wurzel }
  Assign(F, '..\geheim.txt');
  {$I-} Reset(F); {$I+} N := IOResult;
  WriteLn('Reset ausserhalb: ', N);
  {$I-} Rewrite(F); {$I+} N := IOResult;
  WriteLn('Rewrite ausserhalb: ', N);
  Assign(F, 'a/../../geheim.txt');
  {$I-} Reset(F); {$I+} N := IOResult;
  WriteLn('Reset ueber a: ', N);
  { aufraeumen }
  Assign(F, 'a/b/notiz.txt'); Erase(F);
  RmDir('a/b');
  RmDir('a');
  {$I-} ChDir('a'); {$I+} N := IOResult;
  WriteLn('nach RmDir: ', N)
end.
