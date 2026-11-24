program Zahlen;
{ Abnahme Etappe 8: Zahlenraten mit Farben, ClrScr, Eingaben mit ReadLn,
  einer kurzen Animation mit Delay und "Weiter mit Enter" ueber ReadKey.
  Mit dem Argument -fest ist die geheime Zahl reproduzierbar. }
uses Crt;
var
  Geheim, Tipp, Versuche, i: Integer;
  Taste: Char;
begin
  if ParamStr(1) = '-fest' then
    RandSeed := 1
  else
    Randomize;
  repeat
    ClrScr;
    TextColor(Yellow);
    WriteLn('Ich denke mir eine Zahl zwischen 1 und 100.');
    NormVideo;
    Geheim := Random(100) + 1;
    Versuche := 0;
    repeat
      Write('Dein Tipp: ');
      ReadLn(Tipp);
      Inc(Versuche);
      if Tipp < Geheim then
      begin
        TextColor(LightRed);
        WriteLn('zu klein');
        NormVideo
      end
      else if Tipp > Geheim then
      begin
        TextColor(LightBlue);
        WriteLn('zu gross');
        NormVideo
      end
    until Tipp = Geheim;
    TextColor(LightGreen);
    Write('Richtig');
    for i := 1 to 3 do
    begin
      Delay(150);
      Write('!')
    end;
    NormVideo;
    WriteLn(' nach ', Versuche, ' Versuchen.');
    Write('Weiter mit Enter ');
    repeat
      Taste := ReadKey
    until Taste = #13;
    Write('Nochmal (j/n)? ');
    Taste := UpCase(ReadKey);
    WriteLn
  until Taste <> 'J'
end.
