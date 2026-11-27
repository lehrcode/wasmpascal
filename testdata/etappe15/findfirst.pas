program Finden;
{ FindFirst/FindNext mit Mustern und Attributen. Der Host bestimmt die
  Reihenfolge; das Programm sortiert die Namen vor der Ausgabe. }
uses Dos;
var
  F: Text;

procedure Liste(Muster: string; Attr: Word);
var
  S: SearchRec;
  Namen: array[1..20] of string;
  Infos: array[1..20] of string;
  N, I, J: Integer;
  T, Zahl: string;
begin
  N := 0;
  FindFirst(Muster, Attr, S);
  while DosError = 0 do
  begin
    Inc(N);
    Namen[N] := S.Name;
    Str(S.Attr, Zahl);
    T := '/' + Zahl;
    if S.Attr and Directory = 0 then
    begin
      Str(S.Size, Zahl);
      T := T + '/' + Zahl
    end;
    Infos[N] := T;
    FindNext(S)
  end;
  for I := 1 to N - 1 do
    for J := I + 1 to N do
      if Namen[J] < Namen[I] then
      begin
        T := Namen[I]; Namen[I] := Namen[J]; Namen[J] := T;
        T := Infos[I]; Infos[I] := Infos[J]; Infos[J] := T
      end;
  Write(Muster, ':');
  for I := 1 to N do
    Write(' ', Namen[I], Infos[I]);
  WriteLn(' (', DosError, ')')
end;

begin
  Assign(F, 'a.txt'); Rewrite(F); Write(F, 'abc'); Close(F);
  Assign(F, 'b.txt'); Rewrite(F); Close(F);
  Assign(F, 'c.pas'); Rewrite(F); Close(F);
  Assign(F, 'notiz'); Rewrite(F); Close(F);
  Assign(F, '.versteckt.txt'); Rewrite(F); Close(F);
  MkDir('ordner');
  MkDir('ordner.txt');
  Liste('*.txt', 0);
  Liste('?.txt', 0);
  Liste('*.*', 0);
  Liste('*', Directory);
  Liste('*.txt', Hidden);
  Liste('*.TXT', 0);
  Liste('*', AnyFile);
  Liste('*', ReadOnly + SysFile + Archive);
  Liste('ordner.txt', Directory);
  Liste('ordner.txt/../*.pas', 0);
  Liste('gibts.nicht', 0);
  Liste('ordner/*.*', 0);
  Liste('fehlt/*.*', 0)
end.
