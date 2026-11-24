program Aufzaehlung;
type
  TTag = (Mo, Di, Mi, Do_, Fr, Sa, So);
  TWerktag = Mo..Fr;
  TAmpel = (Rot, Gelb, Gruen);
const
  Namen: array[TTag] of string[10] =
    ('Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag', 'Sonntag');
var
  t: TTag;
  w: TWerktag;
  a: TAmpel;
begin
  for t := Mo to So do
  begin
    Write(Ord(t), ' ', Namen[t]);
    if t in [Sa, So] then Write(' (Wochenende)');
    WriteLn
  end;
  w := Fr;
  t := Pred(w);
  WriteLn(Namen[Succ(t)]);
  a := Rot;
  repeat
    case a of
      Rot: WriteLn('Halt');
      Gelb: WriteLn('Achtung');
      Gruen: WriteLn('Fahren')
    end;
    if a = High(TAmpel) then a := Low(TAmpel) else a := Succ(a)
  until a = Rot
end.
